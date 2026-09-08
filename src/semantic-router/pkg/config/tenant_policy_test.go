package config

import "testing"

func TestTenantPolicyResolveInheritsDefaultAndDoesNotRetainTenantID(t *testing.T) {
	defaultCap := 0.08
	tenantCap := 0.01
	cfg := TenantPolicyConfig{
		Enabled: true,
		Default: TenantRoutingPolicy{
			AllowedProviders: []string{"mistral", "openai"},
			MaxEstimatedCost: &defaultCap,
		},
		Tenants: map[string]TenantRoutingPolicy{
			"private-tenant": {DeniedModels: []string{"premium"}, MaxEstimatedCost: &tenantCap},
		},
	}

	resolved := cfg.Resolve("private-tenant")
	if resolved.Source != "tenant" || !resolved.TenantPresent {
		t.Fatalf("unexpected resolution: %+v", resolved)
	}
	if len(resolved.Policy.AllowedProviders) != 2 || len(resolved.Policy.DeniedModels) != 1 {
		t.Fatalf("default policy was not inherited: %+v", resolved.Policy)
	}
	if resolved.Policy.MaxEstimatedCost == nil || *resolved.Policy.MaxEstimatedCost != tenantCap {
		t.Fatalf("tenant cost cap was not applied: %+v", resolved.Policy)
	}
}

func TestValidateTenantPolicyRejectsConflictingModelRules(t *testing.T) {
	cfg := &RouterConfig{
		BackendModels: BackendModels{ModelConfig: map[string]ModelParams{"model-a": {}}},
		TenantPolicy: TenantPolicyConfig{
			Enabled: true,
			Version: "policy-v1",
			Default: TenantRoutingPolicy{
				AllowedModels: []string{"model-a"},
				DeniedModels:  []string{"MODEL-A"},
			},
		},
	}

	if err := validateTenantPolicyContracts(cfg); err == nil {
		t.Fatal("expected conflicting model rules to be rejected")
	}
}

func TestValidateTenantPolicyRequiresPricingForCostCap(t *testing.T) {
	cap := 0.01
	cfg := &RouterConfig{TenantPolicy: TenantPolicyConfig{
		Enabled:          true,
		Version:          "policy-v1",
		Currency:         "USD",
		OutputTokenBound: 100,
		Default:          TenantRoutingPolicy{MaxEstimatedCost: &cap},
	}}

	if err := validateTenantPolicyContracts(cfg); err == nil {
		t.Fatal("expected a tenant cost cap without required pricing to be rejected")
	}
}

func TestValidateTenantPolicyRejectsEmptyProvider(t *testing.T) {
	cfg := &RouterConfig{TenantPolicy: TenantPolicyConfig{
		Enabled: true,
		Version: "policy-v1",
		Default: TenantRoutingPolicy{AllowedProviders: []string{" "}},
	}}

	if err := validateTenantPolicyContracts(cfg); err == nil {
		t.Fatal("expected an empty tenant provider to be rejected")
	}
}

func TestModelProviderUsesCanonicalProfileThenTagFallback(t *testing.T) {
	cfg := &RouterConfig{
		BackendModels: BackendModels{
			ModelConfig: map[string]ModelParams{
				"canonical": {PreferredEndpoints: []string{"canonical-endpoint"}},
				"tagged":    {Tags: []string{"tier:cheap", "provider:Mistral"}},
			},
			VLLMEndpoints: []VLLMEndpoint{{Name: "canonical-endpoint", ProviderProfileName: "canonical-endpoint"}},
			ProviderProfiles: map[string]ProviderProfile{
				"canonical-endpoint": {Type: "OpenAI"},
			},
		},
	}

	if got := cfg.ModelProvider("canonical"); got != "openai" {
		t.Fatalf("canonical provider = %q", got)
	}
	if got := cfg.ModelProvider("tagged"); got != "mistral" {
		t.Fatalf("tag provider = %q", got)
	}
}

func TestModelProviderPrefersVendorTagOverWireProtocol(t *testing.T) {
	cfg := &RouterConfig{BackendModels: BackendModels{
		ModelConfig: map[string]ModelParams{
			"grok": {PreferredEndpoints: []string{"grok-endpoint"}, Tags: []string{"provider:xai"}},
		},
		VLLMEndpoints: []VLLMEndpoint{{Name: "grok-endpoint", ProviderProfileName: "grok-endpoint"}},
		ProviderProfiles: map[string]ProviderProfile{
			"grok-endpoint": {Type: "openai"},
		},
	}}

	if got := cfg.ModelProvider("grok"); got != "xai" {
		t.Fatalf("vendor provider = %q", got)
	}
}
