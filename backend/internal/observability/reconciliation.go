package observability

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
)

const metricNamespace = "pelican_mc_router"

type ReconciliationMetrics struct {
	reconciliations      *prometheus.CounterVec
	duration             prometheus.Histogram
	lastSuccessTimestamp prometheus.Gauge
	consecutiveFailures  prometheus.Gauge
	inProgress           prometheus.Gauge
}

func NewRegistry() (*prometheus.Registry, *ReconciliationMetrics, error) {
	registry := prometheus.NewRegistry()
	metrics, err := NewReconciliationMetrics(registry)
	if err != nil {
		return nil, nil, err
	}

	if err := registry.Register(prometheus.NewGoCollector()); err != nil {
		return nil, nil, fmt.Errorf("register Go collector: %w", err)
	}
	if err := registry.Register(
		prometheus.NewProcessCollector(
			prometheus.ProcessCollectorOpts{},
		),
	); err != nil {
		return nil, nil, fmt.Errorf("register process collector: %w", err)
	}

	return registry, metrics, nil
}

func NewHandler(registry *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

func NewReconciliationMetrics(
	registry prometheus.Registerer,
) (*ReconciliationMetrics, error) {
	metrics := &ReconciliationMetrics{
		reconciliations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: metricNamespace,
				Subsystem: "reconciliation",
				Name:      "total",
				Help:      "Completed reconciliation attempts by outcome.",
			},
			[]string{"result"},
		),
		duration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Subsystem: "reconciliation",
			Name:      "duration_seconds",
			Help:      "Duration of completed reconciliation attempts.",
		}),
		lastSuccessTimestamp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "reconciliation",
			Name:      "last_success_timestamp_seconds",
			Help:      "Unix timestamp of the most recent successful reconciliation.",
		}),
		consecutiveFailures: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "reconciliation",
			Name:      "consecutive_failures",
			Help:      "Consecutive reconciliation failures.",
		}),
		inProgress: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "reconciliation",
			Name:      "in_progress",
			Help:      "Whether reconciliation is currently in progress.",
		}),
	}

	collectors := []prometheus.Collector{
		metrics.reconciliations,
		metrics.duration,
		metrics.lastSuccessTimestamp,
		metrics.consecutiveFailures,
		metrics.inProgress,
	}
	for _, collector := range collectors {
		if err := registry.Register(collector); err != nil {
			return nil, fmt.Errorf("register reconciliation metric: %w", err)
		}
	}

	for _, result := range []string{"not_configured", "success", "failure"} {
		metrics.reconciliations.WithLabelValues(result).Add(0)
	}

	return metrics, nil
}

func (m *ReconciliationMetrics) ObserveReconciliation(
	status runtime.ReconciliationStatus,
) {
	if status.InProgress {
		m.inProgress.Set(1)
		return
	}

	m.inProgress.Set(0)
	if status.LastOutcome == nil ||
		status.LastStartedAt == nil ||
		status.LastCompletedAt == nil {
		return
	}

	result, ok := metricResult(*status.LastOutcome)
	if !ok {
		return
	}

	duration := status.LastCompletedAt.Sub(*status.LastStartedAt)
	if duration < 0 {
		duration = 0
	}
	m.reconciliations.WithLabelValues(result).Inc()
	m.duration.Observe(duration.Seconds())
	m.consecutiveFailures.Set(float64(status.ConsecutiveFailures))

	if *status.LastOutcome == runtime.ReconciliationOutcomeSuccess {
		m.lastSuccessTimestamp.Set(
			float64(status.LastCompletedAt.UnixNano()) / float64(time.Second),
		)
	}
}

func metricResult(outcome runtime.ReconciliationOutcome) (string, bool) {
	switch outcome {
	case runtime.ReconciliationOutcomeNotConfigured:
		return "not_configured", true
	case runtime.ReconciliationOutcomeSuccess:
		return "success", true
	case runtime.ReconciliationOutcomeFailure:
		return "failure", true
	default:
		return "", false
	}
}
