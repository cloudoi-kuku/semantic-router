package config

import "testing"

func TestFallbackAlgorithmValidation(t *testing.T) {
	refs := []ModelRef{{Model: "cheap"}, {Model: "premium"}}
	valid := &AlgorithmConfig{
		Type: DecisionAlgorithmFallback,
		Fallback: &FallbackAlgorithmConfig{
			MaxAttempts: 2,
			RetryOn:     []string{"rate_limited", "server_error"},
		},
	}
	if err := validateDecisionAlgorithmConfig("resilient", refs, valid); err != nil {
		t.Fatalf("valid fallback rejected: %v", err)
	}
	valid.Fallback.MaxAttempts = 3
	if err := validateDecisionAlgorithmConfig("resilient", refs, valid); err == nil {
		t.Fatal("max_attempts beyond declared modelRefs was accepted")
	}
	valid.Fallback.MaxAttempts = 2
	valid.Fallback.RetryOn = []string{"any_error"}
	if err := validateDecisionAlgorithmConfig("resilient", refs, valid); err == nil {
		t.Fatal("unknown fallback failure class was accepted")
	}
}

func TestProviderReliabilityRuntimeCircuitValidation(t *testing.T) {
	if err := validateProviderReliability("model", ProviderReliability{
		CircuitBreakerFailures: 3,
		CircuitBreakerOpenTime: "30s",
	}); err != nil {
		t.Fatalf("valid circuit policy rejected: %v", err)
	}
	if err := validateProviderReliability("model", ProviderReliability{
		CircuitBreakerFailures: 3,
	}); err == nil {
		t.Fatal("circuit policy without open duration was accepted")
	}
}
