package config

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

var currencyCodePattern = regexp.MustCompile(`^[A-Z]{3}$`)

const (
	ProviderPricingCatalogVersion = "vllm-sr/provider-pricing/v1alpha1"
	PricingUnitPerMillionTokens   = "per_1m_tokens"
)

// validateModelPricingContracts keeps operator-supplied cost metadata safe for
// accounting and cost-aware selection. Pricing remains deployment metadata on
// providers.models[]; routing model cards do not own provider rates.
func validateModelPricingContracts(cfg *RouterConfig) error {
	if cfg == nil {
		return nil
	}
	for modelName, params := range cfg.ModelConfig {
		if err := validateModelPricing(modelName, params.Pricing); err != nil {
			return err
		}
	}
	return nil
}

func validateModelPricing(modelName string, pricing ModelPricing) error {
	currency := strings.TrimSpace(pricing.Currency)
	if currency != "" && !currencyCodePattern.MatchString(currency) {
		return fmt.Errorf(
			"providers.models[%s].pricing.currency must be a three-letter uppercase currency code, got %q",
			modelName,
			pricing.Currency,
		)
	}

	if err := validateModelPricingRates(modelName, pricing); err != nil {
		return err
	}
	return validateModelPricingProvenance(modelName, pricing)
}

func validateModelPricingRates(modelName string, pricing ModelPricing) error {
	rates := []struct {
		name  string
		value float64
	}{
		{name: "prompt_per_1m", value: pricing.PromptPer1M},
		{name: "completion_per_1m", value: pricing.CompletionPer1M},
		{name: "cached_input_per_1m", value: pricing.CachedInputPer1M},
	}
	if pricing.CacheWritePer1M != nil {
		rates = append(rates, struct {
			name  string
			value float64
		}{name: "cache_write_per_1m", value: *pricing.CacheWritePer1M})
	}
	if pricing.ReasoningPer1M != nil {
		rates = append(rates, struct {
			name  string
			value float64
		}{name: "reasoning_per_1m", value: *pricing.ReasoningPer1M})
	}

	for _, rate := range rates {
		if math.IsNaN(rate.value) || math.IsInf(rate.value, 0) || rate.value < 0 {
			return fmt.Errorf(
				"providers.models[%s].pricing.%s must be a finite, non-negative per-million-token rate",
				modelName,
				rate.name,
			)
		}
	}
	return nil
}

func validateModelPricingProvenance(modelName string, pricing ModelPricing) error {
	if pricing.Unit != "" && pricing.Unit != PricingUnitPerMillionTokens {
		return fmt.Errorf("providers.models[%s].pricing.unit must be %q", modelName, PricingUnitPerMillionTokens)
	}
	var effective time.Time
	if pricing.EffectiveAt != "" {
		parsed, err := time.Parse(time.RFC3339, pricing.EffectiveAt)
		if err != nil {
			return fmt.Errorf("providers.models[%s].pricing.effective_at must be RFC3339", modelName)
		}
		effective = parsed
	}
	if pricing.ExpiresAt != "" {
		expires, err := time.Parse(time.RFC3339, pricing.ExpiresAt)
		if err != nil {
			return fmt.Errorf("providers.models[%s].pricing.expires_at must be RFC3339", modelName)
		}
		if !effective.IsZero() && !expires.After(effective) {
			return fmt.Errorf("providers.models[%s].pricing.expires_at must be after effective_at", modelName)
		}
	}
	return nil
}
