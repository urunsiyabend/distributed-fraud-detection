# Distributed Fraud Detection

Real-time fraud detection system built in Go with a hybrid sync/async architecture, Domain-Driven Design, and hexagonal architecture.

## Architecture

```
                    ┌─────────────────────────────────┐
                    │          HTTP :8080              │
                    │  POST /v1/transactions/assess    │
                    └──────────────┬──────────────────┘
                                   │
                    ┌──────────────▼──────────────────┐
                    │        Fast Path (20ms)          │
                    │  VelocityRule + AmountRule +     │
                    │  DeviceRule                      │
                    └──────────────┬──────────────────┘
                                   │
                    ┌──────────────▼──────────────────┐
                    │   Atomic DB Transaction          │
                    │   Assessment + Outbox Events     │
                    └──────────────┬──────────────────┘
                                   │
                    ┌──────────────▼──────────────────┐
                    │       Outbox Poller (1s)         │
                    │   Pending → NATS Publish →       │
                    │   MarkPublished / DLQ            │
                    └──────────────┬──────────────────┘
                                   │
                    ┌──────────────▼──────────────────┐
                    │     Slow Path Workers            │
                    │  NATS Queue Group: fraud-workers │
                    │  LocationRule + PatternRule      │
                    │  Decision override + Webhook     │
                    └─────────────────────────────────┘
```

**Fast path** evaluates lightweight rules synchronously within a 20ms budget and returns immediately. If the deadline is exceeded, the client gets a `202 pending` response.

**Slow path** workers consume events via NATS, run expensive rules (impossible travel detection, ML pattern analysis), and can override the fast path decision.

## Domain Model

| Type | Kind | Description |
|------|------|-------------|
| `Money` | Value Object | Amount + currency with ISO 4217 validation |
| `Coordinate` | Value Object | Lat/lng with Haversine distance calculation |
| `RiskScore` | Value Object | 0-100 score with `IsHighRisk()` / `IsReview()` |
| `Decision` | Value Object | `approved` / `blocked` / `review` |
| `PaymentMethod` | Value Object | `card` / `wire` / `crypto` |
| `RuleResult` | Value Object | Rule name, score, reason, fallback flag |
| `Transaction` | Entity | Full transaction context for rule evaluation |
| `FraudAssessment` | Aggregate Root | Combines rule results, derives decision, emits domain events |

## Rules

All rule scores and thresholds are loaded from a config database at runtime — zero hardcoded values.

| Rule | Path | What it checks |
|------|------|----------------|
| `VelocityRule` | Fast | Transaction frequency per sender (Redis sorted set) |
| `AmountRule` | Fast | Amount threshold + critical multiplier |
| `DeviceRule` | Fast | Unknown or missing device fingerprint |
| `LocationRule` | Slow | Impossible travel via Haversine distance |
| `PatternRule` | Slow | Placeholder for ML-based pattern analysis |

Adding a new rule: implement `domain.Rule` interface (`Name()`, `FallbackScore()`, `Evaluate()`), register in the factory.

## Resilience

- **Circuit breakers** (sony/gobreaker) on Redis and Postgres calls — 5 failures → open, 60s timeout, 1 probe in half-open
- **Fallback scores** — when a rule fails, a configurable fallback score is used instead of failing the entire assessment
- **Outbox pattern** — assessment + events written atomically to Postgres, polled and published to NATS. No event loss even if NATS is down
- **Dead letter queue** — outbox entries and NATS messages that fail 3+ times are moved to DLQ
- **Config cache** — all config loaded at startup, refreshed every 60s in background. Stale cache on refresh failure
- **Idempotency** — `X-Idempotency-Key` header with Redis SETNX (24h TTL)

## Observability

**Metrics** (Prometheus, exposed at `GET /metrics`):

```
fraud_assessment_duration_seconds    histogram   5/10/20/50/100ms buckets
fraud_rule_triggered_total           counter     {rule_name}
fraud_rule_fallback_total            counter     {rule_name}
fraud_decision_total                 counter     {decision}
fraud_config_refresh_total           counter     {status}
fraud_circuit_breaker_transitions    counter     {name, from, to}
fraud_worker_panics_total            counter
fraud_worker_messages_total          counter     {success}
fraud_outbox_pending_total           gauge
fraud_outbox_published_total         counter
fraud_outbox_dead_total              counter
```

**Tracing** (OpenTelemetry → OTLP/gRPC):
- Parent span: `fraud.assess` with transaction attributes
- Child spans: `fraud.rule.{name}` per rule evaluation
- Trace ID propagated to structured logs

**Logging** (`log/slog` JSON to stdout):
- Base fields: `service`, `env`, `trace_id`
- Request lifecycle: start/complete with duration and status
- Panic recovery with stack context

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/transactions/assess` | Submit transaction for fraud assessment |
| `GET` | `/health` | Liveness probe (always 200) |
| `GET` | `/ready` | Readiness probe (config cache loaded?) |
| `GET` | `/metrics` | Prometheus scrape endpoint |

### Request

```json
POST /v1/transactions/assess
X-Idempotency-Key: optional-uuid

{
  "id": "tx-123",
  "amount": 5000.00,
  "currency": "USD",
  "sender_id": "user-1",
  "receiver_id": "user-2",
  "device_id": "device-abc",
  "ip": "203.0.113.42",
  "location": {"lat": 41.0082, "lng": 28.9784},
  "timestamp": "2025-01-15T10:30:00Z",
  "payment_method": "card"
}
```

### Response

```json
HTTP/1.1 202 Accepted

{
  "transaction_id": "tx-123",
  "decision": "approved",
  "risk_score": 35,
  "reasons": ["device abc not recognized for sender user-1"],
  "assessed_at": "2025-01-15T10:30:00.015Z",
  "fast_path": true
}
```

## Project Structure

```
├── main.go                                    # Wire-up and lifecycle
├── internal/
│   ├── api/                                   # HTTP layer
│   │   ├── handler.go                         # Assessment endpoint + outbox atomic write
│   │   ├── middleware.go                       # Tracing, RequestID, Logging, Recovery
│   │   ├── models.go                          # Request/response DTOs + validation
│   │   └── router.go                          # Chi router + middleware chain
│   ├── application/                           # Use cases
│   │   ├── fraud_assessor.go                  # Fast path orchestrator with tracing
│   │   ├── rule_factory.go                    # Builds rules from config
│   │   └── slow_path_assessor.go              # Slow path deep analysis
│   ├── domain/                                # Core business logic
│   │   ├── coordinate.go                      # Haversine distance
│   │   ├── decision.go                        # Decision enum
│   │   ├── events.go                          # Domain events
│   │   ├── fraud_assessment.go                # Aggregate root
│   │   ├── money.go                           # Money value object
│   │   ├── payment_method.go                  # Payment method enum
│   │   ├── ports.go                           # All port interfaces
│   │   ├── risk_score.go                      # Risk score value object
│   │   ├── rule.go                            # Rule interface + metrics
│   │   ├── rule_result.go                     # Rule result + fallback
│   │   ├── transaction.go                     # Transaction entity
│   │   └── rules/                             # Rule implementations
│   │       ├── amount.go
│   │       ├── device.go
│   │       ├── location.go
│   │       ├── pattern.go
│   │       └── velocity.go
│   ├── infrastructure/                        # Adapters
│   │   ├── config/async_cache.go              # In-memory config with background refresh
│   │   ├── messaging/
│   │   │   ├── nats_publisher.go              # NATS event publisher
│   │   │   ├── nats_consumer.go               # Queue group consumer with DLQ
│   │   │   └── outbox_poller.go               # Polls outbox table → NATS
│   │   ├── observability/
│   │   │   ├── logger.go                      # slog JSON with trace correlation
│   │   │   ├── metrics.go                     # Prometheus counters/histograms
│   │   │   └── tracer.go                      # OTLP gRPC exporter
│   │   ├── postgres/
│   │   │   ├── assessment_repository.go       # Assessment persistence
│   │   │   ├── config_repository.go           # Config source + bulk load
│   │   │   ├── device_repository.go           # Known device lookup
│   │   │   ├── outbox_repository.go           # Outbox CRUD with FOR UPDATE SKIP LOCKED
│   │   │   ├── unit_of_work.go                # DB transaction abstraction
│   │   │   └── migrations/
│   │   │       └── 002_create_outbox.sql
│   │   ├── redis/
│   │   │   ├── idempotency_store.go           # SETNX with 24h TTL
│   │   │   └── transaction_counter.go         # Sorted set velocity counting
│   │   └── resilience/
│   │       └── circuit_breaker.go             # gobreaker wrappers
│   └── worker/
│       └── pool.go                            # NATS consumer worker pool
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go |
| HTTP Router | chi |
| Database | PostgreSQL |
| Cache / Counters | Redis |
| Messaging | NATS |
| Metrics | Prometheus |
| Tracing | OpenTelemetry (OTLP/gRPC) |
| Logging | log/slog (JSON) |
| Circuit Breaker | sony/gobreaker |

## Prerequisites

- Go 1.22+
- PostgreSQL
- Redis
- NATS
- OpenTelemetry Collector (optional, for tracing)

## Graceful Shutdown

Signal → stop HTTP server → drain worker pool → outbox poller stops → close NATS/Redis/Postgres → flush traces.

## License

MIT
