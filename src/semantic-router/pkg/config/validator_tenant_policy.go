package config

import (
	"fmt"
	"strings"
)

func validateTenantPolicyContracts(cfg *RouterConfig) error {
	if cfg == nil {
		return nil
	}
	policy := cfg.TenantPolicy
	if !policy.Enabled {
		return nil
	}
	if err := validateTenantPolicySettings(policy); err != nil {
		return err
	}
	if err := validateTenantRoutingPolicy(cfg, "default", policy.Default); err != nil {
		return err
	}
	for tenantID, tenant := range policy.Tenants {
		if strings.TrimSpace(tenantID) == "" {
			return fmt.Errorf("global.services.tenant_policy.tenants contains an empty tenant identifier")
		}
		if err := validateTenantRoutingPolicy(cfg, "tenants."+tenantID, tenant); err != nil {
			return err
		}
	}
	return nil
}

func validateTenantPolicySettings(policy TenantPolicyConfig) error {
	if strings.TrimSpace(policy.Version) == "" {
		return fmt.Errorf("global.services.tenant_policy.version is required when tenant policy is enabled")
	}
	if strings.TrimSpace(policy.GetTenantIDHeader()) == "" {
		return fmt.Errorf("global.services.tenant_policy.tenant_id_header cannot be empty")
	}
	if policy.RequireCurrentPricing && !policy.RequirePricing {
		return fmt.Errorf("global.services.tenant_policy.require_current_pricing requires require_pricing")
	}
	if !tenantPolicyHasCostCap(policy) {
		return nil
	}
	return validateTenantCostCapSettings(policy)
}

func validateTenantCostCapSettings(policy TenantPolicyConfig) error {
	currency := strings.TrimSpace(policy.Currency)
	if len(currency) != 3 || currency != strings.ToUpper(currency) {
		return fmt.Errorf("global.services.tenant_policy.currency must be a three-letter currency code when a cost cap is configured")
	}
	if !policy.RequirePricing {
		return fmt.Errorf("global.services.tenant_policy.require_pricing must be true when a cost cap is configured")
	}
	if policy.OutputTokenBound <= 0 {
		return fmt.Errorf("global.services.tenant_policy.output_token_bound must be greater than zero when a cost cap is configured")
	}
	if policy.ReasoningTokenBound < 0 {
		return fmt.Errorf("global.services.tenant_policy.reasoning_token_bound cannot be negative")
	}
	return nil
}

func tenantPolicyHasCostCap(policy TenantPolicyConfig) bool {
	if policy.Default.MaxEstimatedCost != nil {
		return true
	}
	for _, tenant := range policy.Tenants {
		if tenant.MaxEstimatedCost != nil {
			return true
		}
	}
	return false
}

func validateTenantRoutingPolicy(cfg *RouterConfig, path string, policy TenantRoutingPolicy) error {
	prefix := "global.services.tenant_policy." + path
	if policy.MaxEstimatedCost != nil && *policy.MaxEstimatedCost <= 0 {
		return fmt.Errorf("%s.max_estimated_cost must be greater than zero", prefix)
	}
	if overlap := firstStringSetOverlap(policy.AllowedModels, policy.DeniedModels); overlap != "" {
		return fmt.Errorf("%s model %q cannot be both allowed and denied", prefix, overlap)
	}
	if overlap := firstStringSetOverlap(policy.AllowedProviders, policy.DeniedProviders); overlap != "" {
		return fmt.Errorf("%s provider %q cannot be both allowed and denied", prefix, overlap)
	}
	for _, model := range append(append([]string(nil), policy.AllowedModels...), policy.DeniedModels...) {
		if _, ok := cfg.ModelConfig[strings.TrimSpace(model)]; !ok {
			return fmt.Errorf("%s references unknown model %q", prefix, model)
		}
	}
	for _, provider := range append(append([]string(nil), policy.AllowedProviders...), policy.DeniedProviders...) {
		if strings.TrimSpace(provider) == "" {
			return fmt.Errorf("%s contains an empty provider identifier", prefix)
		}
	}
	return nil
}

func firstStringSetOverlap(left, right []string) string {
	values := make(map[string]struct{}, len(left))
	for _, value := range left {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			values[value] = struct{}{}
		}
	}
	for _, value := range right {
		value = strings.ToLower(strings.TrimSpace(value))
		if _, ok := values[value]; ok && value != "" {
			return value
		}
	}
	return ""
}
