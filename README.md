# Distributed Fraud Detection

Microservices-based fraud detection system built in Go. Two services communicate via gRPC with distributed tracing, event-driven async processing, and resilience patterns throughout.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        transaction-service :8081                     │
│  POST /v1/transactions  GET /v1/transactions/{id}  POST /{id}/mfa  │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ gRPC (25ms timeout, circuit breaker)
                               │ trace context propagated
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         fraud-detection :8080 / :50051              │
│                                                                     │
│  ┌─────────────────────┐    ┌────────────────────────────────────┐  │
│  │    Fast Path (20ms)  │    │         Slow Path (async)          │  │
│  │                      │    │                                    │  │
│  │  VelocityRule (Redis)│    │  LocationRule (Haversine)          │  │
│  │  AmountRule (config) │    │  PatternRule (ML placeholder)      │  │
│  │  DeviceRule (Redis)  │    │                                    │  │
│  └──────────┬───────────┘    └──────────────┬─────────────────────┘  │
│             │                               │                        │
│             ▼                               │                        │
│  ┌──────────────────────┐                   │                        │
│  │  Outbox (atomic write)│──── NATS ────────┘                        │
│  │  Assessment + Events  │     poller                                │
│  └──────────────────────┘                                            │
└─────────────────────────────────────────────────────────────────────┘
         │              │              │              │
    ┌────┴────┐   ┌────┴────┐   ┌────┴────┐   ┌────┴────┐
    │Postgres │   │  Redis   │   │  NATS   │   │ Jaeger  │
    │(PgBouncer)│ │ (cache)  │   │(events) │   │(traces) │
    └─────────┘   └─────────┘   └─────────┘   └─────────┘
```

## Services

### transaction-service

Manages transaction lifecycle. Creates transactions, calls fraud-detection via gRPC, applies fraud decision, handles MFA flow.

```
POST /v1/transactions       Create transaction + fraud check
GET  /v1/transactions/{id}  Query transaction status
POST /v1/transactions/{id}/mfa  Approve after MFA verification
GET  /health                Liveness probe
```

**Transaction state machine:**
```
created → pending_fraud_check → approved → completed
                              → blocked
                              → review → pending_mfa → approved
                                                     → blocked
```

When the fraud service is unavailable, the circuit breaker returns `review` with MFA required as safe default.

### fraud-detection

Evaluates transactions against configurable fraud rules. Exposes both REST and gRPC APIs.

```
POST /v1/transactions/assess    REST fraud assessment
GET  /health                    Liveness probe
GET  /ready                     Readiness probe
GET  /metrics                   Prometheus scrape
gRPC :50051                     fraud.v1.FraudService/Assess
```

**Fast path (20ms budget):** VelocityRule + AmountRule + DeviceRule run synchronously. Result persisted atomically with outbox events.

**Slow path (async):** Workers consume from NATS, run expensive rules (LocationRule, PatternRule), can override fast path decision.

### fraud-worker

Same codebase as fraud-detection, separate binary. Runs NATS consumer pool + outbox poller without HTTP/gRPC servers.

## Service Communication

```
transaction-service ──gRPC──→ fraud-detection
                              (JSON codec, OTel propagation)

fraud-detection ──outbox──→ NATS ──→ fraud-worker
                              (at-least-once delivery)
```

Distributed tracing propagates across gRPC boundaries via W3C TraceContext. A single request produces a trace spanning both services:

```
transaction-service: fraud.check
  └── gRPC client: fraud.v1.FraudService/Assess
        └── fraud-detection: gRPC server
              └── fraud.assess
                    ├── fraud.rule.velocity
                    ├── fraud.rule.amount
                    └── fraud.rule.device
              └── handler.saveAtomically
                    ├── db.begin
                    ├── db.save_assessment
                    ├── db.save_outbox
                    └── db.commit
```

## Domain Model

### Fraud Detection

| Type | Kind | Description |
|------|------|-------------|
| `Money` | Value Object | Amount + currency with ISO 4217 validation |
| `Coordinate` | Value Object | Lat/lng with Haversine distance calculation |
| `RiskScore` | Value Object | 0-100, `IsHighRisk()` (>70), `IsReview()` (40-70) |
| `Decision` | Value Object | `approved` / `blocked` / `review` |
| `RuleResult` | Value Object | Rule name, score, reason, fallback flag |
| `FraudAssessment` | Aggregate Root | Derives decision from rules, emits domain events |

### Transaction

| Type | Kind | Description |
|------|------|-------------|
| `Transaction` | Entity | Full state machine with fraud decision tracking |
| `TransactionStatus` | Value Object | 8 states: created, pending_fraud_check, approved, blocked, review, pending_mfa, completed, failed |

## Rules

All scores and thresholds loaded from config database at runtime.

| Rule | Path | What it checks | Trigger |
|------|------|----------------|---------|
| `VelocityRule` | Fast | Transaction frequency per sender | Redis sorted set ZCOUNT |
| `AmountRule` | Fast | Amount threshold + 3x critical | Config threshold |
| `DeviceRule` | Fast | Unknown or missing device | Redis cache (read-through from Postgres) |
| `LocationRule` | Slow | Impossible travel | Haversine distance |
| `PatternRule` | Slow | ML-based pattern analysis | Placeholder |

Adding a new rule: implement `domain.Rule` interface (`Name()`, `FallbackScore()`, `Evaluate()`), register in factory.

## Resilience

| Pattern | Implementation | Behavior |
|---------|---------------|----------|
| **Circuit breaker** | sony/gobreaker | Ratio-based trip (>50% failure, min 20 req), 5s recovery, 3 probes |
| **Fallback scores** | Per-rule config | Rule fails → configurable fallback score, assessment continues |
| **Outbox pattern** | Postgres + poller | Atomic write (assessment + events), 1s poll, 3 retries → DLQ |
| **Connection pooling** | PgBouncer | Transaction mode, 20 server connections, 1000 client connections |
| **Device cache warmup** | Startup bulk load | Postgres → Redis at boot, read-through on miss |
| **Config cache** | Async refresh | Loaded at startup, 60s background refresh, stale on failure |
| **Idempotency** | Redis SETNX | `X-Idempotency-Key` header, 24h TTL |
| **gRPC fallback** | CB in transaction-service | Fraud service down → `review` + MFA required |

### Chaos Test Results

| Scenario | Error Impact | Data Loss | Recovery |
|----------|-------------|-----------|----------|
| NATS kill | 0% errors | 0 | Outbox buffers, auto-reconnect |
| Redis kill | 0% errors, fallback scores | 0 | StatefulSet + CB 5s recovery |
| CPU stress | 0% errors | 0 | HPA 2→4 pods in 25s |
| Postgres kill | 500 errors during downtime | 0 | PVC preserves data, ~30s recovery |

## Observability

**Metrics** (Prometheus at `/metrics`):

| Metric | Type | Labels |
|--------|------|--------|
| `fraud_assessment_duration_seconds` | histogram | 5/10/20/50/100ms buckets |
| `fraud_rule_triggered_total` | counter | `rule_name` |
| `fraud_rule_fallback_total` | counter | `rule_name` |
| `fraud_decision_total` | counter | `decision` |
| `fraud_circuit_breaker_transitions` | counter | `name`, `from`, `to` |
| `fraud_outbox_pending_total` | gauge | - |
| `fraud_outbox_published_total` | counter | - |

**Tracing** (OpenTelemetry → Jaeger):
- Distributed traces across transaction-service → fraud-detection via gRPC
- W3C TraceContext propagation
- Span attributes: transaction ID, amount, decision, risk score, rule results

**Logging** (`log/slog` JSON to stdout):
- Base fields: `service`, `env`, `trace_id`
- Request lifecycle with duration and status code
- Panic recovery with stack context

## Project Structure

```
.
├── fraud-detection/                        # Fraud detection service
│   ├── cmd/
│   │   ├── api/main.go                     # HTTP :8080 + gRPC :50051
│   │   └── worker/main.go                  # NATS consumer + outbox poller
│   ├── internal/
│   │   ├── api/                            # HTTP handlers, middleware, router
│   │   ├── application/                    # FraudAssessor, RuleFactory, SlowPathAssessor
│   │   ├── domain/                         # Value objects, entities, aggregate root
│   │   │   └── rules/                      # Amount, Device, Location, Pattern, Velocity
│   │   ├── grpc/                           # gRPC FraudService server
│   │   └── infrastructure/
│   │       ├── config/                     # Async config cache
│   │       ├── messaging/                  # NATS publisher/consumer, outbox poller
│   │       ├── observability/              # Logger, metrics, tracer
│   │       ├── postgres/                   # Repositories, UoW, outbox, migrations
│   │       ├── redis/                      # Device cache, velocity counter, idempotency
│   │       ├── resilience/                 # Circuit breakers
│   │       └── testutil/                   # Testcontainers helpers
│   ├── Dockerfile
│   └── go.mod
│
├── transaction-service/                    # Transaction lifecycle service
│   ├── cmd/api/main.go                     # HTTP :8081
│   ├── internal/
│   │   ├── api/                            # HTTP handlers, router
│   │   ├── application/                    # TransactionService (create, get, MFA)
│   │   ├── domain/                         # Transaction entity + state machine
│   │   └── infrastructure/
│   │       ├── grpc/                       # Fraud gRPC client (CB, timeout, tracing)
│   │       └── postgres/                   # TransactionRepository, migrations
│   ├── Dockerfile
│   └── go.mod
│
├── proto/                                  # Shared gRPC contract
│   └── fraud/v1/
│       ├── fraud.proto                     # Service definition
│       ├── fraud_grpc.pb.go                # Server/client stubs
│       ├── types.go                        # Request/response types
│       └── codec.go                        # JSON codec for gRPC
│
├── k8s/                                    # Kubernetes manifests
│   ├── kustomization.yaml                  # Kustomize entry point
│   ├── fraud-api.yaml                      # 2 replicas, HPA, LoadBalancer
│   ├── fraud-worker.yaml                   # 2 replicas
│   ├── transaction-service.yaml            # 2 replicas
│   ├── postgres.yaml                       # StatefulSet + PVC + seed
│   ├── transaction-db.yaml                 # Separate Postgres for transactions
│   ├── redis.yaml                          # StatefulSet + AOF
│   ├── nats.yaml                           # JetStream enabled
│   ├── pgbouncer.yaml                      # Connection pooler
│   ├── hpa.yaml                            # CPU 50%, 2-10 replicas
│   └── chaos/                              # Chaos Mesh scenarios
│       ├── redis-kill.yaml
│       ├── postgres-kill.yaml
│       ├── nats-kill.yaml
│       ├── cpu-stress.yaml
│       └── network-delay.yaml
│
├── load-tests/                             # k6 load tests + monitoring
│   ├── baseline.js                         # 10 VU, 30s, p95<30ms
│   ├── stress.js                           # 200 VU ramp, p95<50ms
│   ├── spike.js                            # 10→500→10 VU
│   ├── soak.js                             # 50 VU, 10min, goroutine/memory tracking
│   ├── data.js                             # 70% approved / 20% review / 10% blocked
│   ├── seed.sql                            # Schema + config + 100 known devices
│   ├── pgbouncer.ini                       # PgBouncer config
│   ├── prometheus.yml                      # Scrape config
│   └── grafana-dashboard.json              # 8-panel dashboard
│
├── docker-compose.test.yml                 # Full local stack (10 services)
├── go.work                                 # Go workspace (3 modules)
└── README.md
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.25 |
| HTTP Router | chi |
| Inter-service | gRPC (JSON codec) |
| Database | PostgreSQL (2 instances) |
| Connection Pool | PgBouncer |
| Cache | Redis |
| Messaging | NATS |
| Metrics | Prometheus + Grafana |
| Tracing | OpenTelemetry + Jaeger |
| Logging | log/slog (JSON) |
| Circuit Breaker | sony/gobreaker |
| Load Testing | k6 |
| Chaos Testing | Chaos Mesh |
| Container Orchestration | Kubernetes + Kustomize |
| Integration Testing | testcontainers-go |

## Getting Started

### Docker Compose (local development)

```bash
docker compose -f docker-compose.test.yml up -d
```

Services available at:
- **Transaction API**: http://localhost:8081
- **Fraud API**: http://localhost:8080
- **Jaeger**: http://localhost:16686
- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090

### Quick test

```bash
# Create a transaction (calls fraud-detection via gRPC)
curl -X POST http://localhost:8081/v1/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":200,"currency":"USD","sender_id":"user-1","receiver_id":"user-2","device_id":"known-device-1","ip":"1.2.3.4","lat":41,"lng":29,"payment_method":"card"}'

# → {"id":"...","status":"approved","fraud_decision":"approved","fraud_score":0}

# High-risk transaction
curl -X POST http://localhost:8081/v1/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":5000,"currency":"USD","sender_id":"user-1","receiver_id":"user-2","device_id":"known-device-1","ip":"1.2.3.4","lat":41,"lng":29,"payment_method":"card"}'

# → {"id":"...","status":"blocked","fraud_decision":"blocked","fraud_score":80}
```

### Kubernetes

```bash
kubectl apply -k k8s/
```

### Load tests

```bash
k6 run load-tests/baseline.js
k6 run load-tests/stress.js
```

### Chaos tests (requires Chaos Mesh)

```bash
helm install chaos-mesh chaos-mesh/chaos-mesh -n chaos-testing --create-namespace
kubectl apply -f k8s/chaos/redis-kill.yaml
```

## Performance

Tested with 100-200 concurrent users on Docker Desktop (single node).

| Metric | Baseline (10 VU) | Stress (100 VU) |
|--------|------------------|-----------------|
| p95 latency | 24ms | 35ms |
| p50 latency | 8ms | 9ms |
| Throughput | 833 req/s | 679 req/s |
| Error rate | 0% | 0% |
| Assessment time | <5ms (p99) | <5ms (p99) |

## License

MIT
