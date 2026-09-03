package config

import (
	"fmt"
	"strings"
)

func validateDecisionCapabilityContracts(cfg *RouterConfig, decision Decision) error {
	if len(decision.RequiredCapabilities) == 0 {
		return nil
	}
	if err := validateRequiredCapabilityIDs(decision); err != nil {
		return err
	}
	knownCandidates, eligibleCandidates := countCapabilityEligibleCandidates(cfg, decision)
	if knownCandidates > 0 && eligibleCandidates == 0 {
		return fmt.Errorf(
			"decision %q: no modelRef satisfies required_capabilities %v",
			decision.Name,
			decision.RequiredCapabilities,
		)
	}
	if decision.Algorithm != nil && decision.Algorithm.MinimumCandidates > eligibleCandidates && knownCandidates > 0 {
		return fmt.Errorf(
			"decision %q: required_capabilities leave %d eligible modelRefs, below algorithm.minimum_candidates=%d",
			decision.Name,
			eligibleCandidates,
			decision.Algorithm.MinimumCandidates,
		)
	}
	return nil
}

func validateRequiredCapabilityIDs(decision Decision) error {
	seen := make(map[string]struct{}, len(decision.RequiredCapabilities))
	for _, raw := range decision.RequiredCapabilities {
		capability := strings.TrimSpace(raw)
		if capability == "" {
			return fmt.Errorf("decision %q: required_capabilities cannot contain an empty value", decision.Name)
		}
		if capability != raw || !IsSupportedRequiredModelCapability(capability) {
			return fmt.Errorf("decision %q: unsupported required capability %q", decision.Name, raw)
		}
		if _, ok := seen[capability]; ok {
			return fmt.Errorf("decision %q: duplicate required capability %q", decision.Name, capability)
		}
		seen[capability] = struct{}{}
	}
	return nil
}

func countCapabilityEligibleCandidates(cfg *RouterConfig, decision Decision) (int, int) {
	knownCandidates := 0
	eligibleCandidates := 0
	for _, ref := range decision.ModelRefs {
		params, ok := cfg.ModelConfig[strings.TrimSpace(ref.Model)]
		if !ok {
			continue
		}
		knownCandidates++
		if len(MissingModelCapabilities(params, decision.RequiredCapabilities)) == 0 {
			eligibleCandidates++
		}
	}
	return knownCandidates, eligibleCandidates
}
