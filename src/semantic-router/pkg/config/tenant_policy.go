package config

import "strings"

const TenantPolicyContractVersion = "vllm-sr/tenant-policy/v1alpha1"

// TenantPolicyConfig applies trusted tenant identity to model and provider
// eligibility before economic ranking. Tenant identifiers are configuration
// keys only and must not be copied into metrics or public routing evidence.
type TenantPolicyConfig struct {
	Enabled               bool                           `yaml:"enabled" json:"enabled"`
	Version               string                         `yaml:"version,omitempty" json:"version,omitempty"`
	TenantIDHeader        string                         `yaml:"tenant_id_header,omitempty" json:"tenant_id_header,omitempty"`
	RequireTenant         bool                           `yaml:"require_tenant" json:"require_tenant"`
	Currency              string                         `yaml:"currency,omitempty" json:"currency,omitempty"`
	OutputTokenBound      int                            `yaml:"output_token_bound,omitempty" json:"output_token_bound,omitempty"`
	ReasoningTokenBound   int                            `yaml:"reasoning_token_bound,omitempty" json:"reasoning_token_bound,omitempty"`
	RequirePricing        bool                           `yaml:"require_pricing" json:"require_pricing"`
	RequireCurrentPricing bool                           `yaml:"require_current_pricing" json:"require_current_pricing"`
	Default               TenantRoutingPolicy            `yaml:"default,omitempty" json:"default,omitempty"`
	Tenants               map[string]TenantRoutingPolicy `yaml:"tenants,omitempty" json:"tenants,omitempty"`
}

// TenantRoutingPolicy is provider-neutral policy applied to one tenant. Empty
// allowlists permit every configured value; deny lists always win.
type TenantRoutingPolicy struct {
	AllowedModels    []string `yaml:"allowed_models,omitempty" json:"allowed_models,omitempty"`
	DeniedModels     []string `yaml:"denied_models,omitempty" json:"denied_models,omitempty"`
	AllowedProviders []string `yaml:"allowed_providers,omitempty" json:"allowed_providers,omitempty"`
	DeniedProviders  []string `yaml:"denied_providers,omitempty" json:"denied_providers,omitempty"`
	MaxEstimatedCost *float64 `yaml:"max_estimated_cost,omitempty" json:"max_estimated_cost,omitempty"`
}

// ResolvedTenantPolicy is an internal, content-free policy resolution result.
type ResolvedTenantPolicy struct {
	Enabled       bool
	TenantPresent bool
	Source        string
	Policy        TenantRoutingPolicy
}

func (c TenantPolicyConfig) GetTenantIDHeader() string {
	if header := strings.TrimSpace(c.TenantIDHeader); header != "" {
		return header
	}
	return "x-authz-tenant-id"
}

// Resolve returns the effective policy without retaining the tenant identifier.
// Explicit tenant entries replace non-empty default allow/deny lists and the
// cost cap while unspecified fields inherit the default.
func (c TenantPolicyConfig) Resolve(tenantID string) ResolvedTenantPolicy {
	tenantID = strings.TrimSpace(tenantID)
	result := ResolvedTenantPolicy{
		Enabled:       c.Enabled,
		TenantPresent: tenantID != "",
		Source:        "disabled",
		Policy:        cloneTenantRoutingPolicy(c.Default),
	}
	if !c.Enabled {
		return result
	}
	result.Source = "default"
	if tenantID == "" {
		return result
	}
	if override, ok := c.Tenants[tenantID]; ok {
		result.Source = "tenant"
		result.Policy = mergeTenantRoutingPolicy(result.Policy, override)
	}
	return result
}

func mergeTenantRoutingPolicy(base, override TenantRoutingPolicy) TenantRoutingPolicy {
	if override.AllowedModels != nil {
		base.AllowedModels = append([]string(nil), override.AllowedModels...)
	}
	if override.DeniedModels != nil {
		base.DeniedModels = append([]string(nil), override.DeniedModels...)
	}
	if override.AllowedProviders != nil {
		base.AllowedProviders = append([]string(nil), override.AllowedProviders...)
	}
	if override.DeniedProviders != nil {
		base.DeniedProviders = append([]string(nil), override.DeniedProviders...)
	}
	if override.MaxEstimatedCost != nil {
		value := *override.MaxEstimatedCost
		base.MaxEstimatedCost = &value
	}
	return base
}

func cloneTenantRoutingPolicy(policy TenantRoutingPolicy) TenantRoutingPolicy {
	return mergeTenantRoutingPolicy(TenantRoutingPolicy{}, policy)
}

// ModelProvider resolves a product-facing provider identity without exposing
// backend access details. A provider:* model tag is authoritative because a
// vendor may use another provider's wire protocol. The backend profile type is
// only a compatibility fallback for untagged models.
func (c *RouterConfig) ModelProvider(model string) string {
	if c == nil {
		return ""
	}
	params, ok := c.ModelConfig[strings.TrimSpace(model)]
	if !ok {
		return ""
	}
	for _, tag := range params.Tags {
		if provider, found := strings.CutPrefix(strings.ToLower(strings.TrimSpace(tag)), "provider:"); found {
			return strings.TrimSpace(provider)
		}
	}
	for _, endpoint := range params.PreferredEndpoints {
		profile, err := c.GetProviderProfileForEndpoint(endpoint)
		if err == nil && profile != nil {
			if provider := strings.ToLower(strings.TrimSpace(profile.Type)); provider != "" {
				return provider
			}
		}
	}
	return ""
}
