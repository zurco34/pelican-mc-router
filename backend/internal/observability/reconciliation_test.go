package observability

import (
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_model/go"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
)

func TestReconciliationMetricsInitialValues(t *testing.T) {
	registry := prometheus.NewRegistry()
	if _, err := NewReconciliationMetrics(registry); err != nil {
		t.Fatal(err)
	}
	assertMetricValue(t, registry, "pelican_mc_router_reconciliation_in_progress", 0)
	assertMetricValue(t, registry, "pelican_mc_router_reconciliation_consecutive_failures", 0)
	assertMetricValue(t, registry, "pelican_mc_router_reconciliation_last_success_timestamp_seconds", 0)
	assertCounterValue(t, registry, "not_configured", 0)
	assertCounterValue(t, registry, "success", 0)
	assertCounterValue(t, registry, "failure", 0)
}

func TestReconciliationMetricsObservations(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewReconciliationMetrics(registry)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 7, 29, 6, 45, 0, 0, time.UTC)
	completed := started.Add(125 * time.Millisecond)
	success := runtime.ReconciliationOutcomeSuccess
	failure := runtime.ReconciliationOutcomeFailure
	notConfigured := runtime.ReconciliationOutcomeNotConfigured
	secret := "api-key-secret"

	metrics.ObserveReconciliation(runtime.ReconciliationStatus{InProgress: true, LastOutcome: &success})
	assertMetricValue(t, registry, "pelican_mc_router_reconciliation_in_progress", 1)
	assertCounterValue(t, registry, "success", 0)

	metrics.ObserveReconciliation(runtime.ReconciliationStatus{LastOutcome: &success, LastStartedAt: &started, LastCompletedAt: &completed})
	assertMetricValue(t, registry, "pelican_mc_router_reconciliation_in_progress", 0)
	assertCounterValue(t, registry, "success", 1)
	lastSuccess := float64(completed.UnixNano()) / float64(time.Second)
	assertMetricValue(t, registry, "pelican_mc_router_reconciliation_last_success_timestamp_seconds", lastSuccess)

	metrics.ObserveReconciliation(runtime.ReconciliationStatus{LastOutcome: &failure, LastStartedAt: &started, LastCompletedAt: &completed, ConsecutiveFailures: 2, LastError: &secret})
	assertCounterValue(t, registry, "failure", 1)
	assertMetricValue(t, registry, "pelican_mc_router_reconciliation_consecutive_failures", 2)
	assertMetricValue(t, registry, "pelican_mc_router_reconciliation_last_success_timestamp_seconds", lastSuccess)
	recoveryCompleted := completed.Add(time.Second)
	metrics.ObserveReconciliation(runtime.ReconciliationStatus{LastOutcome: &success, LastStartedAt: &completed, LastCompletedAt: &recoveryCompleted})
	assertCounterValue(t, registry, "success", 2)
	assertMetricValue(t, registry, "pelican_mc_router_reconciliation_consecutive_failures", 0)
	lastSuccess = float64(recoveryCompleted.UnixNano()) / float64(time.Second)
	assertMetricValue(t, registry, "pelican_mc_router_reconciliation_last_success_timestamp_seconds", lastSuccess)

	metrics.ObserveReconciliation(runtime.ReconciliationStatus{LastOutcome: &notConfigured, LastStartedAt: &started, LastCompletedAt: &completed})
	assertCounterValue(t, registry, "not_configured", 1)
	assertMetricValue(t, registry, "pelican_mc_router_reconciliation_consecutive_failures", 0)
	assertMetricValue(t, registry, "pelican_mc_router_reconciliation_last_success_timestamp_seconds", lastSuccess)
	assertHistogram(t, registry, 4, 1.375)
	recorder := httptest.NewRecorder()
	NewHandler(registry).ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	if strings.Contains(recorder.Body.String(), secret) {
		t.Fatalf("secret exposed in complete metrics response: %s", recorder.Body.String())
	}
}

func TestReconciliationMetricsRejectsUnknownOutcome(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewReconciliationMetrics(registry)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 7, 29, 6, 45, 0, 0, time.UTC)
	completed := started.Add(time.Second)
	success := runtime.ReconciliationOutcomeSuccess
	metrics.ObserveReconciliation(runtime.ReconciliationStatus{LastOutcome: &success, LastStartedAt: &started, LastCompletedAt: &completed, ConsecutiveFailures: 3})
	before := reconciliationMetricSnapshot(t, registry)
	unknown := runtime.ReconciliationOutcome("secret-outcome")
	metrics.ObserveReconciliation(runtime.ReconciliationStatus{LastOutcome: &unknown, LastStartedAt: &started, LastCompletedAt: &completed})
	assertReconciliationMetricSnapshot(t, registry, before)
	if strings.Contains(metricsText(t, registry), "secret-outcome") {
		t.Fatal("unknown outcome was exposed in metrics")
	}
}

func TestReconciliationMetricsRejectsMalformedCompletions(t *testing.T) {
	tests := []struct {
		name   string
		status func(time.Time, time.Time, *runtime.ReconciliationOutcome) runtime.ReconciliationStatus
	}{
		{
			name: "nil outcome",
			status: func(started, completed time.Time, _ *runtime.ReconciliationOutcome) runtime.ReconciliationStatus {
				return runtime.ReconciliationStatus{LastStartedAt: &started, LastCompletedAt: &completed}
			},
		},
		{
			name: "nil started at",
			status: func(_ time.Time, completed time.Time, outcome *runtime.ReconciliationOutcome) runtime.ReconciliationStatus {
				return runtime.ReconciliationStatus{LastOutcome: outcome, LastCompletedAt: &completed}
			},
		},
		{
			name: "nil completed at",
			status: func(started, _ time.Time, outcome *runtime.ReconciliationOutcome) runtime.ReconciliationStatus {
				return runtime.ReconciliationStatus{LastOutcome: outcome, LastStartedAt: &started}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := prometheus.NewRegistry()
			metrics, err := NewReconciliationMetrics(registry)
			if err != nil {
				t.Fatal(err)
			}
			started := time.Date(2026, 7, 29, 6, 45, 0, 0, time.UTC)
			completed := started.Add(time.Second)
			success := runtime.ReconciliationOutcomeSuccess
			metrics.ObserveReconciliation(runtime.ReconciliationStatus{LastOutcome: &success, LastStartedAt: &started, LastCompletedAt: &completed, ConsecutiveFailures: 4})
			before := reconciliationMetricSnapshot(t, registry)
			metrics.ObserveReconciliation(test.status(started, completed, &success))
			assertReconciliationMetricSnapshot(t, registry, before)
			assertMetricValue(t, registry, "pelican_mc_router_reconciliation_in_progress", 0)
		})
	}
}

func TestReconciliationMetricsClampsNegativeDuration(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewReconciliationMetrics(registry)
	if err != nil {
		t.Fatal(err)
	}
	completed := time.Date(2026, 7, 29, 6, 45, 0, 0, time.UTC)
	started := completed.Add(time.Second)
	metrics.ObserveReconciliation(runtime.ReconciliationStatus{LastOutcome: outcome(runtime.ReconciliationOutcomeSuccess), LastStartedAt: &started, LastCompletedAt: &completed})
	assertHistogram(t, registry, 1, 0)
}

func TestReconciliationMetricsConcurrentObservations(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewReconciliationMetrics(registry)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC()
	completed := started.Add(time.Millisecond)
	success := runtime.ReconciliationOutcomeSuccess
	status := runtime.ReconciliationStatus{LastOutcome: &success, LastStartedAt: &started, LastCompletedAt: &completed}
	var group sync.WaitGroup
	for range 50 {
		group.Add(1)
		go func() { defer group.Done(); metrics.ObserveReconciliation(status) }()
	}
	group.Wait()
	assertCounterValue(t, registry, "success", 50)
}

func outcome(value runtime.ReconciliationOutcome) *runtime.ReconciliationOutcome {
	return &value
}

func assertMetricValue(t *testing.T, registry *prometheus.Registry, name string, want float64) {
	t.Helper()
	metric := metricFamily(t, registry, name).Metric[0]
	if got := metric.GetGauge().GetValue(); got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

func assertCounterValue(t *testing.T, registry *prometheus.Registry, result string, want float64) {
	t.Helper()
	for _, metric := range metricFamily(t, registry, "pelican_mc_router_reconciliation_total").Metric {
		if metric.GetLabel()[0].GetValue() == result && metric.GetCounter().GetValue() == want {
			return
		}
	}
	t.Fatalf("counter result %q = %v, want %v", result, 0, want)
}

func assertHistogram(t *testing.T, registry *prometheus.Registry, wantCount uint64, wantSum float64) {
	t.Helper()
	histogram := metricFamily(t, registry, "pelican_mc_router_reconciliation_duration_seconds").Metric[0].GetHistogram()
	if histogram.GetSampleCount() != wantCount || histogram.GetSampleSum() != wantSum {
		t.Fatalf("histogram = count %d sum %v, want count %d sum %v", histogram.GetSampleCount(), histogram.GetSampleSum(), wantCount, wantSum)
	}
}

func metricFamily(t *testing.T, registry *prometheus.Registry, name string) *io_prometheus_client.MetricFamily {
	t.Helper()
	families := gather(t, registry)
	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	t.Fatalf("metric family %q not found", name)
	return nil
}

func gather(t *testing.T, registry *prometheus.Registry) []*io_prometheus_client.MetricFamily {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	return families
}

type reconciliationMetricsSnapshot struct {
	notConfigured      float64
	success            float64
	failure            float64
	histogramCount     uint64
	histogramSum       float64
	lastSuccess        float64
	consecutiveFailure float64
}

func reconciliationMetricSnapshot(t *testing.T, registry *prometheus.Registry) reconciliationMetricsSnapshot {
	t.Helper()
	histogram := metricFamily(t, registry, "pelican_mc_router_reconciliation_duration_seconds").Metric[0].GetHistogram()
	return reconciliationMetricsSnapshot{
		notConfigured:      counterValue(t, registry, "not_configured"),
		success:            counterValue(t, registry, "success"),
		failure:            counterValue(t, registry, "failure"),
		histogramCount:     histogram.GetSampleCount(),
		histogramSum:       histogram.GetSampleSum(),
		lastSuccess:        gaugeValue(t, registry, "pelican_mc_router_reconciliation_last_success_timestamp_seconds"),
		consecutiveFailure: gaugeValue(t, registry, "pelican_mc_router_reconciliation_consecutive_failures"),
	}
}

func assertReconciliationMetricSnapshot(t *testing.T, registry *prometheus.Registry, want reconciliationMetricsSnapshot) {
	t.Helper()
	if got := reconciliationMetricSnapshot(t, registry); got != want {
		t.Fatalf("metrics changed: got %+v, want %+v", got, want)
	}
}

func counterValue(t *testing.T, registry *prometheus.Registry, result string) float64 {
	t.Helper()
	for _, metric := range metricFamily(t, registry, "pelican_mc_router_reconciliation_total").Metric {
		if metric.GetLabel()[0].GetValue() == result {
			return metric.GetCounter().GetValue()
		}
	}
	t.Fatalf("counter result %q not found", result)
	return 0
}

func gaugeValue(t *testing.T, registry *prometheus.Registry, name string) float64 {
	t.Helper()
	return metricFamily(t, registry, name).Metric[0].GetGauge().GetValue()
}

func metricsText(t *testing.T, registry *prometheus.Registry) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	NewHandler(registry).ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	return recorder.Body.String()
}
