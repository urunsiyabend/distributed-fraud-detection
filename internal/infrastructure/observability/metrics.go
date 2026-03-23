package observability

import (
	"fmt"

	"distributed-fraud-detection/internal/domain"

	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusMetrics struct {
	assessmentDuration prometheus.Histogram
	ruleTriggered      *prometheus.CounterVec
	ruleFallback       *prometheus.CounterVec
	decisionTotal      *prometheus.CounterVec
	configRefresh      *prometheus.CounterVec
	cbTransitions      *prometheus.CounterVec
	workerPanics       prometheus.Counter
	workerMessages     *prometheus.CounterVec
	workerDLQ          prometheus.Counter
	outboxPending      prometheus.Gauge
	outboxPublished    prometheus.Counter
	outboxDead         prometheus.Counter
}

func NewPrometheusMetrics(reg prometheus.Registerer) (*PrometheusMetrics, error) {
	m := &PrometheusMetrics{
		assessmentDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "fraud_assessment_duration_seconds",
			Help:    "Duration of fraud assessment in seconds.",
			Buckets: []float64{0.005, 0.01, 0.02, 0.05, 0.1},
		}),
		ruleTriggered: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fraud_rule_triggered_total",
			Help: "Total number of triggered fraud rules.",
		}, []string{"rule_name"}),
		ruleFallback: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fraud_rule_fallback_total",
			Help: "Total number of rule fallbacks due to errors.",
		}, []string{"rule_name"}),
		decisionTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fraud_decision_total",
			Help: "Total fraud decisions by type.",
		}, []string{"decision"}),
		configRefresh: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fraud_config_refresh_total",
			Help: "Total config refresh attempts by status.",
		}, []string{"status"}),
		cbTransitions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fraud_circuit_breaker_transitions",
			Help: "Circuit breaker state transitions.",
		}, []string{"name", "from", "to"}),
		workerPanics: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fraud_worker_panics_total",
			Help: "Total worker panics recovered.",
		}),
		workerMessages: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fraud_worker_messages_total",
			Help: "Total worker messages processed by status.",
		}, []string{"success"}),
		workerDLQ: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fraud_worker_dlq_total",
			Help: "Total messages sent to dead letter queue.",
		}),
		outboxPending: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "fraud_outbox_pending_total",
			Help: "Current number of pending outbox entries in batch.",
		}),
		outboxPublished: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fraud_outbox_published_total",
			Help: "Total outbox entries successfully published.",
		}),
		outboxDead: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fraud_outbox_dead_total",
			Help: "Total outbox entries moved to dead letter.",
		}),
	}

	collectors := []prometheus.Collector{
		m.assessmentDuration,
		m.ruleTriggered,
		m.ruleFallback,
		m.decisionTotal,
		m.configRefresh,
		m.cbTransitions,
		m.workerPanics,
		m.workerMessages,
		m.workerDLQ,
		m.outboxPending,
		m.outboxPublished,
		m.outboxDead,
	}

	for _, c := range collectors {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}

	return m, nil
}

func (m *PrometheusMetrics) AssessmentDuration(seconds float64) {
	m.assessmentDuration.Observe(seconds)
}

func (m *PrometheusMetrics) DecisionMade(decision domain.Decision) {
	m.decisionTotal.WithLabelValues(string(decision)).Inc()
}

func (m *PrometheusMetrics) RuleTriggered(ruleName string) {
	m.ruleTriggered.WithLabelValues(ruleName).Inc()
}

func (m *PrometheusMetrics) RuleFallback(ruleName string) {
	m.ruleFallback.WithLabelValues(ruleName).Inc()
}

func (m *PrometheusMetrics) ConfigRefreshSuccess() {
	m.configRefresh.WithLabelValues("success").Inc()
}

func (m *PrometheusMetrics) ConfigRefreshError() {
	m.configRefresh.WithLabelValues("error").Inc()
}

func (m *PrometheusMetrics) CircuitBreakerStateChange(name, from, to string) {
	m.cbTransitions.WithLabelValues(name, from, to).Inc()
}

func (m *PrometheusMetrics) WorkerPanic(_ int) {
	m.workerPanics.Inc()
}

func (m *PrometheusMetrics) WorkerMessageProcessed(success bool) {
	m.workerMessages.WithLabelValues(fmt.Sprintf("%t", success)).Inc()
}

func (m *PrometheusMetrics) WorkerDLQ(_ string) {
	m.workerDLQ.Inc()
}

func (m *PrometheusMetrics) OutboxPending(count int) {
	m.outboxPending.Set(float64(count))
}

func (m *PrometheusMetrics) OutboxPublished() {
	m.outboxPublished.Inc()
}

func (m *PrometheusMetrics) OutboxDead() {
	m.outboxDead.Inc()
}
