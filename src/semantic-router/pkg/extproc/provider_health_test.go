package extproc

import (
	"testing"
	"time"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
)

func TestProviderCircuitOpensAndSuccessResetsIt(t *testing.T) {
	router := &OpenAIRouter{Config: &config.RouterConfig{BackendModels: config.BackendModels{ModelConfig: map[string]config.ModelParams{
		"primary": {Reliability: config.ProviderReliability{CircuitBreakerFailures: 2, CircuitBreakerOpenTime: "1m"}},
	}}}}
	ctx := &RequestContext{VSRSelectedModel: "primary"}
	router.observeProviderHealth(ctx, 503)
	if router.modelCircuitOpen("primary", time.Now()) {
		t.Fatal("circuit opened before threshold")
	}
	router.observeProviderHealth(ctx, 429)
	if !router.modelCircuitOpen("primary", time.Now()) {
		t.Fatal("circuit did not open at threshold")
	}
	eligibility := router.eligibleModelRefs(
		[]config.ModelRef{{Model: "primary"}, {Model: "secondary"}}, nil, 0,
	)
	if len(eligibility.eligible) != 1 || eligibility.eligible[0].Model != "secondary" ||
		len(eligibility.exclusions) != 1 || eligibility.exclusions[0].Reasons[0] != "circuit_open" {
		t.Fatalf("health-aware eligibility = %#v", eligibility)
	}
	router.observeProviderHealth(ctx, 200)
	if router.modelCircuitOpen("primary", time.Now()) {
		t.Fatal("successful probe did not reset circuit")
	}
}
