package modelpricing

import "testing"

func TestEstimatedCostUsesIndependentTokenBounds(t *testing.T) {
	reasoningRate := 20.0
	cost := EstimatedCost(Estimate{
		InputTokens: 1000, OutputTokens: 500, ReasoningTokens: 250,
	}, Rates{PromptPer1M: 2, CompletionPer1M: 10, ReasoningPer1M: &reasoningRate})
	if cost != 0.012 {
		t.Fatalf("estimated cost = %v, want 0.012", cost)
	}
}

func TestEstimatedCostFallsBackToCompletionRateForReasoning(t *testing.T) {
	cost := EstimatedCost(Estimate{ReasoningTokens: 1000}, Rates{CompletionPer1M: 12})
	if cost != 0.012 {
		t.Fatalf("estimated cost = %v, want 0.012", cost)
	}
}
