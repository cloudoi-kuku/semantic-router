package extproc

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/observability/logging"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/services"
)

var errNoContextEligibleDecisionModel = errors.New("no decision model can satisfy the request context")

type modelEligibilityResult struct {
	eligible   []config.ModelRef
	exclusions []services.ModelEligibilityExclusion
}

// contextEligibleModelRefs applies only contracts that can be established from
// local configuration. Missing or zero context-window metadata remains
// eligible so a partial inventory does not turn into an accidental outage.
func (r *OpenAIRouter) contextEligibleModelRefs(
	refs []config.ModelRef,
	contextTokens int,
) (eligible []config.ModelRef, excluded int) {
	if len(refs) == 0 {
		return nil, 0
	}
	eligible = make([]config.ModelRef, 0, len(refs))
	for _, ref := range refs {
		if r.modelRefExceedsContextWindow(ref, contextTokens) {
			excluded++
			continue
		}
		eligible = append(eligible, ref)
	}
	return eligible, excluded
}

func (r *OpenAIRouter) eligibleModelRefs(
	refs []config.ModelRef,
	requiredCapabilities []string,
	contextTokens int,
) modelEligibilityResult {
	result := modelEligibilityResult{eligible: make([]config.ModelRef, 0, len(refs))}
	for _, ref := range refs {
		exclusion := services.ModelEligibilityExclusion{Model: ref.Model}
		if r.modelCircuitOpen(ref.Model, time.Now()) {
			exclusion.Reasons = append(exclusion.Reasons, "circuit_open")
		}
		if r.modelRefExceedsContextWindow(ref, contextTokens) {
			exclusion.Reasons = append(exclusion.Reasons, "context_window")
		}
		if len(requiredCapabilities) > 0 {
			params, ok := r.Config.ModelConfig[strings.TrimSpace(ref.Model)]
			if !ok {
				exclusion.MissingCapabilities = append([]string(nil), requiredCapabilities...)
			} else {
				exclusion.MissingCapabilities = config.MissingModelCapabilities(params, requiredCapabilities)
			}
			if len(exclusion.MissingCapabilities) > 0 {
				exclusion.Reasons = append(exclusion.Reasons, "missing_capabilities")
			}
		}
		if len(exclusion.Reasons) > 0 {
			result.exclusions = append(result.exclusions, exclusion)
			continue
		}
		result.eligible = append(result.eligible, ref)
	}
	return result
}

func (r *OpenAIRouter) contextEligibleDecisionModelRefs(
	refs []config.ModelRef,
	decisionName string,
	requiredCapabilities []string,
	contextTokens int,
	ctx *RequestContext,
) ([]config.ModelRef, error) {
	tenantID := headerValueCI(ctx, r.Config.TenantPolicy.GetTenantIDHeader())
	resolvedTenant, tenantEvidence := r.resolveTenantPolicy(tenantID)
	ctx.VSRTenantPolicy = tenantEvidence
	tenantResult := r.tenantPolicyEligibleModelRefs(refs, resolvedTenant)
	result := r.eligibleModelRefs(tenantResult.eligible, requiredCapabilities, contextTokens)
	result.exclusions = append(tenantResult.exclusions, result.exclusions...)
	if len(result.eligible) == 0 && len(result.exclusions) > 0 {
		return nil, fmt.Errorf(
			"%w: every configured candidate for decision %q failed tenant, context, or capability eligibility",
			errNoContextEligibleDecisionModel,
			decisionName,
		)
	}
	ctx.VSREligibleModelRefs = cloneModelRefs(result.eligible)
	if len(result.exclusions) > 0 {
		logging.ComponentEvent("extproc", "decision_models_eligibility_filtered", map[string]interface{}{
			"request_id":            ctx.RequestID,
			"decision":              decisionName,
			"context_tokens":        contextTokens,
			"required_capabilities": append([]string(nil), requiredCapabilities...),
			"excluded_candidates":   len(result.exclusions),
			"eligible_candidates":   len(result.eligible),
		})
	}
	return result.eligible, nil
}

func (r *OpenAIRouter) modelRefExceedsContextWindow(ref config.ModelRef, contextTokens int) bool {
	return r.modelNameExceedsContextWindow(ref.Model, contextTokens)
}

// decisionRouteActionDestination resolves a matched decision's route action.
// The action is terminal: the destination, or an eligible decision candidate
// when the destination cannot satisfy the request context, overrides a
// caller-pinned model so a detected prompt attack can never fall back to the
// caller's choice. With no eligible safe model at all the request fails
// closed.
func (r *OpenAIRouter) decisionRouteActionDestination(
	decision *config.Decision,
	ctx *RequestContext,
) (string, bool, error) {
	if decision == nil || decision.Action == nil ||
		decision.Action.Type != config.DecisionActionRoute {
		return "", false, nil
	}
	destination := strings.TrimSpace(decision.Action.Destination)
	if destination == "" {
		return "", false, nil
	}
	resolvedTenant, evidence := r.resolveTenantPolicy(
		headerValueCI(ctx, r.Config.TenantPolicy.GetTenantIDHeader()),
	)
	ctx.VSRTenantPolicy = evidence
	destinationTenant := r.tenantPolicyEligibleModelRefs(
		[]config.ModelRef{{Model: destination}}, resolvedTenant,
	)
	destinationBudget := r.requestBudgetEligibleModelRefs(
		destinationTenant.eligible, effectiveTenantRequestBudget(decision.RequestBudget, r.Config.TenantPolicy, resolvedTenant.Policy),
		ctx.VSRContextTokenCount, ctx.VSROutputTokenBound, ctx.VSRReasoningTokenBound, time.Now().UTC(),
	)
	if !r.modelNameExceedsContextWindow(destination, ctx.VSRContextTokenCount) &&
		len(r.modelNameMissingCapabilities(destination, decision.RequiredCapabilities)) == 0 &&
		len(destinationBudget.eligible) > 0 {
		logging.ComponentEvent("extproc", "route_action_applied", map[string]interface{}{
			"request_id":  ctx.RequestID,
			"decision":    decision.Name,
			"destination": destination,
		})
		return destination, true, nil
	}
	tenantEligibility := r.tenantPolicyEligibleModelRefs(decision.ModelRefs, resolvedTenant)
	eligibility := r.eligibleModelRefs(
		tenantEligibility.eligible,
		decision.RequiredCapabilities,
		ctx.VSRContextTokenCount,
	)
	budgetEligibility := r.requestBudgetEligibleModelRefs(
		eligibility.eligible, effectiveTenantRequestBudget(decision.RequestBudget, r.Config.TenantPolicy, resolvedTenant.Policy),
		ctx.VSRContextTokenCount, ctx.VSROutputTokenBound, ctx.VSRReasoningTokenBound, time.Now().UTC(),
	)
	eligibility.eligible = budgetEligibility.eligible
	if len(eligibility.eligible) > 0 {
		logging.ComponentEvent("extproc", "route_action_destination_ineligible", map[string]interface{}{
			"request_id":     ctx.RequestID,
			"decision":       decision.Name,
			"destination":    destination,
			"fallback":       eligibility.eligible[0].Model,
			"context_tokens": ctx.VSRContextTokenCount,
		})
		return eligibility.eligible[0].Model, true, nil
	}
	return "", false, fmt.Errorf(
		"%w: route action destination %q and every candidate of decision %q failed context or capability eligibility",
		errNoContextEligibleDecisionModel,
		destination,
		decision.Name,
	)
}

func (r *OpenAIRouter) modelNameMissingCapabilities(model string, required []string) []string {
	if r == nil || r.Config == nil || len(required) == 0 {
		return nil
	}
	params, ok := r.Config.ModelConfig[strings.TrimSpace(model)]
	if !ok {
		return append([]string(nil), required...)
	}
	return config.MissingModelCapabilities(params, required)
}

func validateMinimumEligibleDecisionModels(
	decision *config.Decision,
	eligible []config.ModelRef,
	contextTokens int,
) error {
	if decision == nil || decision.Algorithm == nil ||
		decision.Algorithm.MinimumCandidates <= 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(eligible))
	for _, ref := range eligible {
		model := strings.TrimSpace(ref.Model)
		if model == "" {
			continue
		}
		seen[model+"\x00"+strings.TrimSpace(ref.LoRAName)] = struct{}{}
	}
	if len(seen) >= decision.Algorithm.MinimumCandidates {
		return nil
	}
	return fmt.Errorf(
		"%w: decision %q requires at least %d eligible candidates for %d request tokens, got %d",
		errNoContextEligibleDecisionModel,
		decision.Name,
		decision.Algorithm.MinimumCandidates,
		contextTokens,
		len(seen),
	)
}

func (r *OpenAIRouter) modelNameExceedsContextWindow(model string, contextTokens int) bool {
	if r == nil || r.Config == nil || contextTokens <= 0 {
		return false
	}
	params, ok := r.Config.ModelConfig[strings.TrimSpace(model)]
	return ok && params.ContextWindowSize > 0 && contextTokens > params.ContextWindowSize
}

// contextIneligibleAlgorithmModelCount covers explicit multi-model control
// models that can bypass decision.modelRefs during execution. The algorithm is
// rejected instead of rewritten so planner, judge, and synthesis semantics stay
// visible and deterministic.
func (r *OpenAIRouter) contextIneligibleAlgorithmModelCount(
	decision *config.Decision,
	contextTokens int,
) int {
	if decision == nil || decision.Algorithm == nil || contextTokens <= 0 {
		return 0
	}
	models := explicitAlgorithmModels(decision.Algorithm)
	seen := make(map[string]struct{}, len(models))
	count := 0
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		if r.modelNameExceedsContextWindow(model, contextTokens) {
			count++
		}
	}
	return count
}

func (r *OpenAIRouter) capabilityIneligibleAlgorithmModelCount(
	decision *config.Decision,
) int {
	if decision == nil || decision.Algorithm == nil || len(decision.RequiredCapabilities) == 0 {
		return 0
	}
	seen := make(map[string]struct{})
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
		if len(r.modelNameMissingCapabilities(model, decision.RequiredCapabilities)) > 0 {
			count++
		}
	}
	return count
}

func explicitAlgorithmModels(algorithm *config.AlgorithmConfig) []string {
	if algorithm == nil {
		return nil
	}
	var models []string
	if fusion := algorithm.Fusion; fusion != nil {
		models = append(models, fusion.Model)
		models = append(models, fusion.AnalysisModels...)
	}
	if remom := algorithm.ReMoM; remom != nil {
		models = append(models, remom.SynthesisModel)
	}
	if workflows := algorithm.Workflows; workflows != nil {
		models = append(models, workflows.Planner.Model, workflows.Final.Model)
		for _, role := range workflows.Roles {
			models = append(models, role.Models...)
		}
	}
	return models
}
