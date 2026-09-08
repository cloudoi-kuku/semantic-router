package config

import "fmt"

func validateFallbackAlgorithmConfig(
	decisionName string,
	modelRefs []ModelRef,
	cfg *FallbackAlgorithmConfig,
) error {
	if cfg == nil {
		return fmt.Errorf("decision '%s': algorithm.type=fallback requires algorithm.fallback configuration", decisionName)
	}
	if cfg.MaxAttempts < 1 || cfg.MaxAttempts > len(modelRefs) {
		return fmt.Errorf(
			"decision '%s', algorithm.fallback.max_attempts must be between 1 and the %d declared modelRefs",
			decisionName,
			len(modelRefs),
		)
	}
	allowed := map[string]struct{}{
		"transport_error": {}, "timeout": {}, "rate_limited": {},
		"server_error": {}, "invalid_response": {},
	}
	for _, trigger := range cfg.RetryOn {
		if _, ok := allowed[trigger]; !ok {
			return fmt.Errorf("decision '%s', algorithm.fallback.retry_on contains unsupported failure class %q", decisionName, trigger)
		}
	}
	return nil
}
