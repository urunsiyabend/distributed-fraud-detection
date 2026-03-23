package observability

import (
	"context"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type Logger struct {
	inner *slog.Logger
}

func NewLogger(service, env string) *Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler).With(
		slog.String("service", service),
		slog.String("env", env),
	)

	return &Logger{inner: logger}
}

func (l *Logger) LogAssessment(ctx context.Context, transactionID string, decision string, riskScore int, duration time.Duration) {
	l.inner.InfoContext(ctx, "assessment completed",
		slog.String("transaction_id", transactionID),
		slog.String("decision", decision),
		slog.Int("risk_score", riskScore),
		slog.Float64("duration_ms", float64(duration.Milliseconds())),
		traceAttr(ctx),
	)
}

func (l *Logger) LogRuleFallback(ctx context.Context, ruleName string, err error) {
	l.inner.WarnContext(ctx, "rule fallback activated",
		slog.String("rule_name", ruleName),
		slog.String("error", err.Error()),
		traceAttr(ctx),
	)
}

func (l *Logger) LogCircuitBreakerChange(ctx context.Context, name, from, to string) {
	l.inner.WarnContext(ctx, "circuit breaker state change",
		slog.String("breaker_name", name),
		slog.String("from", from),
		slog.String("to", to),
		traceAttr(ctx),
	)
}

func traceAttr(ctx context.Context) slog.Attr {
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		return slog.String("trace_id", sc.TraceID().String())
	}
	return slog.Attr{}
}
