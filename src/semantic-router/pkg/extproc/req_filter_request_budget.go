package extproc

import (
	"fmt"
	"strings"
	"time"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/modelpricing"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/services"
)

type requestBudgetResult struct {
	eligible   []config.ModelRef
	exclusions []services.ModelEligibilityExclusion
	evaluation *services.RequestCostEvaluation
}

func (r *OpenAIRouter) applyRuntimeRequestBudget(
	refs []config.ModelRef,
	decision *config.Decision,
	decisionName string,
	ctx *RequestContext,
) ([]config.ModelRef, error) {
	if decision == nil {
		return refs, nil
	}
	result := r.requestBudgetEligibleModelRefs(
		refs, decision.RequestBudget, ctx.VSRContextTokenCount,
		ctx.VSROutputTokenBound, ctx.VSRReasoningTokenBound, time.Now().UTC(),
	)
	result = r.boundFallbackChain(result, decision)
	ctx.VSREligibleModelRefs = cloneModelRefs(result.eligible)
	if len(result.eligible) == 0 {
		return nil, fmt.Errorf(
			"%w: every configured candidate for decision %q failed request-budget eligibility",
			errNoContextEligibleDecisionModel, decisionName,
		)
	}
	return result.eligible, nil
}

func (r *OpenAIRouter) requestBudgetEligibleModelRefs(
	refs []config.ModelRef,
	budget *config.RequestBudget,
	inputTokens int,
	requestedOutputTokens int,
	requestedReasoningTokens int,
	now time.Time,
) requestBudgetResult {
	result := requestBudgetResult{eligible: append([]config.ModelRef(nil), refs...)}
	if budget == nil {
		return result
	}
	outputTokens := budget.OutputTokenBound
	if requestedOutputTokens > 0 {
		outputTokens = requestedOutputTokens
	}
	reasoningTokens := budget.ReasoningTokenBound
	if requestedReasoningTokens > 0 {
		reasoningTokens = requestedReasoningTokens
	}
	result.eligible = make([]config.ModelRef, 0, len(refs))
	result.evaluation = &services.RequestCostEvaluation{
		CatalogVersion:  config.ProviderPricingCatalogVersion,
		Currency:        budget.Currency,
		InputTokens:     max(inputTokens, 0),
		OutputTokens:    outputTokens,
		ReasoningTokens: reasoningTokens,
		MaxCost:         budget.MaxEstimatedCost,
		Candidates:      make([]services.CandidateCostEstimate, 0, len(refs)),
	}
	for _, ref := range refs {
		estimate, exclusion := r.estimateCandidateCost(ref, budget, inputTokens, outputTokens, reasoningTokens, now)
		result.evaluation.Candidates = append(result.evaluation.Candidates, estimate)
		if exclusion != nil {
			result.exclusions = append(result.exclusions, *exclusion)
			continue
		}
		result.eligible = append(result.eligible, ref)
	}
	return result
}

func (r *OpenAIRouter) estimateCandidateCost(
	ref config.ModelRef,
	budget *config.RequestBudget,
	inputTokens int,
	outputTokens int,
	reasoningTokens int,
	now time.Time,
) (services.CandidateCostEstimate, *services.ModelEligibilityExclusion) {
	model := strings.TrimSpace(ref.Model)
	estimate := services.CandidateCostEstimate{Model: model, Eligible: true, Status: "within_budget", MaxProviderAttempts: 1}
	if reliability, ok := r.Config.GetProviderReliability(model); ok {
		estimate.MaxProviderAttempts += reliability.RetryCount
	}
	exclusion := &services.ModelEligibilityExclusion{Model: model}
	pricing, configured := r.Config.GetFullModelPricing(model)
	if !configured {
		estimate.Status = "pricing_missing"
		if budget.RequirePricing {
			estimate.Eligible = false
			exclusion.Reasons = append(exclusion.Reasons, "pricing_missing")
			return estimate, exclusion
		}
		return estimate, nil
	}
	estimate.PriceVersion = pricing.Version
	estimate.PriceSource = pricing.Source
	estimate.EffectiveAt = pricing.EffectiveAt
	estimate.ExpiresAt = pricing.ExpiresAt
	if !strings.EqualFold(pricing.Currency, budget.Currency) {
		estimate.Status = "currency_mismatch"
		estimate.Eligible = false
		exclusion.Reasons = append(exclusion.Reasons, "currency_mismatch")
		return estimate, exclusion
	}
	if budget.RequireCurrentPricing && !pricingIsCurrent(pricing, now) {
		estimate.Status = "pricing_stale"
		estimate.Eligible = false
		exclusion.Reasons = append(exclusion.Reasons, "pricing_stale")
		return estimate, exclusion
	}
	estimate.SingleAttemptEstimatedCost = modelpricing.EstimatedCost(modelpricing.Estimate{
		InputTokens: inputTokens, OutputTokens: outputTokens, ReasoningTokens: reasoningTokens,
	}, modelPricingRates(pricing))
	estimate.EstimatedCost = estimate.SingleAttemptEstimatedCost * float64(estimate.MaxProviderAttempts)
	if estimate.EstimatedCost > budget.MaxEstimatedCost {
		estimate.Status = "budget_exceeded"
		estimate.Eligible = false
		exclusion.Reasons = append(exclusion.Reasons, "budget_exceeded")
		return estimate, exclusion
	}
	return estimate, nil
}

func (r *OpenAIRouter) boundFallbackChain(
	result requestBudgetResult,
	decision *config.Decision,
) requestBudgetResult {
	if decision == nil || decision.Algorithm == nil ||
		decision.Algorithm.Type != config.DecisionAlgorithmFallback ||
		decision.Algorithm.Fallback == nil {
		return result
	}
	maxAttempts := decision.Algorithm.Fallback.MaxAttempts
	bounded := make([]config.ModelRef, 0, min(maxAttempts, len(result.eligible)))
	cumulative := 0.0
	for _, ref := range result.eligible {
		estimate := candidateEstimateForModel(result.evaluation, ref.Model)
		reason := fallbackExclusionReason(
			len(bounded), maxAttempts, cumulative, estimate, decision.RequestBudget,
		)
		if reason != "" {
			result.exclusions = append(result.exclusions, services.ModelEligibilityExclusion{
				Model: ref.Model, Reasons: []string{reason},
			})
			if estimate != nil {
				estimate.Eligible = false
				estimate.Status = reason
			}
			continue
		}
		bounded = append(bounded, ref)
		if estimate != nil {
			cumulative += estimate.EstimatedCost
		}
	}
	result.eligible = bounded
	return result
}

func fallbackExclusionReason(
	selected int,
	maxAttempts int,
	cumulative float64,
	estimate *services.CandidateCostEstimate,
	budget *config.RequestBudget,
) string {
	if selected >= maxAttempts {
		return "fallback_attempt_limit"
	}
	if budget != nil && estimate != nil &&
		cumulative+estimate.EstimatedCost > budget.MaxEstimatedCost {
		return "fallback_chain_budget_exceeded"
	}
	return ""
}

func candidateEstimateForModel(
	evaluation *services.RequestCostEvaluation,
	model string,
) *services.CandidateCostEstimate {
	if evaluation == nil {
		return nil
	}
	for index := range evaluation.Candidates {
		if evaluation.Candidates[index].Model == model {
			return &evaluation.Candidates[index]
		}
	}
	return nil
}

func pricingIsCurrent(pricing config.ModelPricing, now time.Time) bool {
	if strings.TrimSpace(pricing.Version) == "" || strings.TrimSpace(pricing.Source) == "" ||
		pricing.Unit != config.PricingUnitPerMillionTokens || pricing.EffectiveAt == "" || pricing.ExpiresAt == "" {
		return false
	}
	effective, effectiveErr := time.Parse(time.RFC3339, pricing.EffectiveAt)
	expires, expiresErr := time.Parse(time.RFC3339, pricing.ExpiresAt)
	return effectiveErr == nil && expiresErr == nil && !now.Before(effective) && now.Before(expires)
}

func (r *OpenAIRouter) requestBudgetIneligibleAlgorithmModelCount(
	decision *config.Decision,
	inputTokens int,
	outputTokens int,
	reasoningTokens int,
	now time.Time,
) int {
	if decision == nil || decision.Algorithm == nil || decision.RequestBudget == nil {
		return 0
	}
	seen := map[string]struct{}{}
	count := 0
	for _, model := range explicitAlgorithmModels(decision.Algorithm) {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		result := r.requestBudgetEligibleModelRefs(
			[]config.ModelRef{{Model: model}}, decision.RequestBudget,
			inputTokens, outputTokens, reasoningTokens, now,
		)
		if len(result.eligible) == 0 {
			count++
		}
	}
	return count
}
