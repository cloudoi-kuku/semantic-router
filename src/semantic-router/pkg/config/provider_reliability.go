package config

import (
	"fmt"
	"strings"
	"time"
)

const (
	ProviderLBPolicyRoundRobin   = "round_robin"
	ProviderLBPolicyLeastRequest = "least_request"
)

func validateProviderReliability(modelName string, reliability ProviderReliability) error {
	switch strings.TrimSpace(reliability.LBPolicy) {
	case "", ProviderLBPolicyRoundRobin, ProviderLBPolicyLeastRequest:
	default:
		return fmt.Errorf(
			"providers.models[%s].reliability.lb_policy must be %q or %q",
			modelName,
			ProviderLBPolicyRoundRobin,
			ProviderLBPolicyLeastRequest,
		)
	}
	if err := validateProviderRetry(modelName, reliability); err != nil {
		return err
	}
	if err := validateProviderOutlierDetection(modelName, reliability); err != nil {
		return err
	}
	if err := validateProviderHealthCheck(modelName, reliability); err != nil {
		return err
	}
	return validateProviderCircuitBreaker(modelName, reliability)
}

func validateProviderCircuitBreaker(modelName string, reliability ProviderReliability) error {
	if reliability.CircuitBreakerFailures < 0 || reliability.CircuitBreakerFailures > 100 {
		return fmt.Errorf("providers.models[%s].reliability.circuit_breaker_failures must be between 0 and 100", modelName)
	}
	if reliability.CircuitBreakerFailures == 0 && reliability.CircuitBreakerOpenTime != "" {
		return fmt.Errorf("providers.models[%s].reliability.circuit_breaker_open_time requires circuit_breaker_failures", modelName)
	}
	if reliability.CircuitBreakerFailures > 0 {
		if strings.TrimSpace(reliability.CircuitBreakerOpenTime) == "" {
			return fmt.Errorf("providers.models[%s].reliability.circuit_breaker_open_time is required when the runtime circuit breaker is enabled", modelName)
		}
		duration, err := time.ParseDuration(reliability.CircuitBreakerOpenTime)
		if err != nil || duration <= 0 {
			return fmt.Errorf("providers.models[%s].reliability.circuit_breaker_open_time must be a positive duration", modelName)
		}
	}
	return nil
}

// GetProviderReliability returns the canonical reliability policy for a
// logical model. The bool is false when the model is not configured.
func (c *RouterConfig) GetProviderReliability(model string) (ProviderReliability, bool) {
	if c == nil || c.ModelConfig == nil {
		return ProviderReliability{}, false
	}
	params, ok := c.ModelConfig[strings.TrimSpace(model)]
	return params.Reliability, ok
}

func validateProviderRetry(modelName string, reliability ProviderReliability) error {
	if reliability.RetryCount < 0 || reliability.RetryCount > 5 {
		return fmt.Errorf(
			"providers.models[%s].reliability.retry_count must be between 0 and 5",
			modelName,
		)
	}
	if reliability.RetryCount > 0 && strings.TrimSpace(reliability.RetryOn) == "" {
		return fmt.Errorf(
			"providers.models[%s].reliability.retry_on is required when retries are enabled",
			modelName,
		)
	}
	return nil
}

func validateProviderOutlierDetection(
	modelName string,
	reliability ProviderReliability,
) error {
	if reliability.Consecutive5xx < 0 {
		return fmt.Errorf(
			"providers.models[%s].reliability.consecutive_5xx cannot be negative",
			modelName,
		)
	}
	if reliability.MaxEjectionPercent < 0 || reliability.MaxEjectionPercent > 100 {
		return fmt.Errorf(
			"providers.models[%s].reliability.max_ejection_percent must be between 0 and 100",
			modelName,
		)
	}
	if reliability.BaseEjectionTime != "" {
		if _, err := time.ParseDuration(reliability.BaseEjectionTime); err != nil {
			return fmt.Errorf(
				"providers.models[%s].reliability.base_ejection_time is invalid: %w",
				modelName,
				err,
			)
		}
	}
	return nil
}

func validateProviderHealthCheck(
	modelName string,
	reliability ProviderReliability,
) error {
	if reliability.HealthCheckPath != "" &&
		!strings.HasPrefix(reliability.HealthCheckPath, "/") {
		return fmt.Errorf(
			"providers.models[%s].reliability.health_check_path must start with /",
			modelName,
		)
	}
	for field, value := range map[string]string{
		"health_check_interval": reliability.HealthCheckInterval,
		"health_check_timeout":  reliability.HealthCheckTimeout,
	} {
		if value == "" {
			continue
		}
		if _, err := time.ParseDuration(value); err != nil {
			return fmt.Errorf(
				"providers.models[%s].reliability.%s is invalid: %w",
				modelName,
				field,
				err,
			)
		}
	}
	return nil
}
