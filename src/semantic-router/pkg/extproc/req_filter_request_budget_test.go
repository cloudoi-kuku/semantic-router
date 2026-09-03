package extproc

import (
	"testing"
	"time"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
)

func TestRequestBudgetFiltersOverBudgetAndKeepsAlternativeComparison(t *testing.T) {
	router := &OpenAIRouter{Config: &config.RouterConfig{BackendModels: config.BackendModels{
		ModelConfig: map[string]config.ModelParams{
			"cheap":   {Pricing: currentTestPricing(1, 2)},
			"premium": {Pricing: currentTestPricing(10, 40)},
		},
	}}}
	result := router.requestBudgetEligibleModelRefs(
		[]config.ModelRef{{Model: "cheap"}, {Model: "premium"}},
		&config.RequestBudget{Currency: "USD", MaxEstimatedCost: 0.01, OutputTokenBound: 1000, RequirePricing: true, RequireCurrentPricing: true},
		1000, 0, 0, time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC),
	)
	if len(result.eligible) != 1 || result.eligible[0].Model != "cheap" {
		t.Fatalf("eligible = %#v, want cheap only", result.eligible)
	}
	if result.evaluation == nil || len(result.evaluation.Candidates) != 2 {
		t.Fatalf("cost evaluation = %#v, want both alternatives", result.evaluation)
	}
	if result.evaluation.Candidates[1].Status != "budget_exceeded" {
		t.Fatalf("premium status = %q", result.evaluation.Candidates[1].Status)
	}
}

func TestRequestBudgetFailsClosedForStalePricing(t *testing.T) {
	pricing := currentTestPricing(1, 2)
	pricing.ExpiresAt = "2026-08-01T00:00:00Z"
	router := &OpenAIRouter{Config: &config.RouterConfig{BackendModels: config.BackendModels{
		ModelConfig: map[string]config.ModelParams{"stale": {Pricing: pricing}},
	}}}
	result := router.requestBudgetEligibleModelRefs(
		[]config.ModelRef{{Model: "stale"}},
		&config.RequestBudget{Currency: "USD", MaxEstimatedCost: 1, OutputTokenBound: 100, RequirePricing: true, RequireCurrentPricing: true},
		100, 0, 0, time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC),
	)
	if len(result.eligible) != 0 || len(result.exclusions) != 1 || result.exclusions[0].Reasons[0] != "pricing_stale" {
		t.Fatalf("result = %#v, want stale exclusion", result)
	}
}

func currentTestPricing(input, output float64) config.ModelPricing {
	return config.ModelPricing{
		Version: "test-v1", Source: "test-catalog", Unit: config.PricingUnitPerMillionTokens,
		EffectiveAt: "2026-01-01T00:00:00Z", ExpiresAt: "2027-01-01T00:00:00Z",
		Currency: "USD", PromptPer1M: input, CompletionPer1M: output,
	}
}
