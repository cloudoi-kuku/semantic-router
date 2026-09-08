package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	executionEvidenceRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_router_execution_evidence_total",
			Help: "Privacy-safe completed execution evidence by algorithm, fallback use, and quality status.",
		},
		[]string{"algorithm", "fallback_used", "quality_status"},
	)
	executionEvidenceRetries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_router_retries_total",
			Help: "Observed same-model and cross-provider retries by algorithm.",
		},
		[]string{"algorithm", "kind"},
	)
	executionEvidenceSavings = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_router_cost_savings_total",
			Help: "Pricing-derived savings against the configured premium baseline.",
		},
		[]string{"currency"},
	)
)

func RecordExecutionEvidence(
	algorithm string,
	fallbackUsed bool,
	qualityStatus string,
	providerRetries int,
	fallbackRetries int,
	currency string,
	savings float64,
) {
	if algorithm == "" {
		algorithm = "static"
	}
	if qualityStatus == "" {
		qualityStatus = "not_measured"
	}
	fallback := "false"
	if fallbackUsed {
		fallback = "true"
	}
	executionEvidenceRequests.WithLabelValues(algorithm, fallback, qualityStatus).Inc()
	if providerRetries > 0 {
		executionEvidenceRetries.WithLabelValues(algorithm, "provider").Add(float64(providerRetries))
	}
	if fallbackRetries > 0 {
		executionEvidenceRetries.WithLabelValues(algorithm, "fallback").Add(float64(fallbackRetries))
	}
	if savings > 0 {
		if currency == "" {
			currency = "USD"
		}
		executionEvidenceSavings.WithLabelValues(currency).Add(savings)
	}
}
