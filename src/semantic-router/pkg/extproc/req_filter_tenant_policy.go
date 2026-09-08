package extproc

import (
	"fmt"
	"strings"
	"time"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/services"
)

func (r *OpenAIRouter) validateTenantPolicyModel(
	model string,
	baseBudget *config.RequestBudget,
	ctx *RequestContext,
) error {
	resolved, evidence := r.resolveTenantPolicy(
		headerValueCI(ctx, r.Config.TenantPolicy.GetTenantIDHeader()),
	)
	ctx.VSRTenantPolicy = evidence
	if eligibility := r.tenantPolicyEligibleModelRefs(
		[]config.ModelRef{{Model: model}}, resolved,
	); len(eligibility.eligible) == 0 {
		return fmt.Errorf(
			"%w: explicitly requested model %q failed tenant-policy eligibility",
			errNoContextEligibleDecisionModel, model,
		)
	}
	budget := effectiveTenantRequestBudget(baseBudget, r.Config.TenantPolicy, resolved.Policy)
	if eligibility := r.requestBudgetEligibleModelRefs(
		[]config.ModelRef{{Model: model}}, budget,
		ctx.VSRContextTokenCount, ctx.VSROutputTokenBound, ctx.VSRReasoningTokenBound, time.Now().UTC(),
	); len(eligibility.eligible) == 0 {
		return fmt.Errorf(
			"%w: explicitly requested model %q failed request-budget eligibility",
			errNoContextEligibleDecisionModel, model,
		)
	}
	return nil
}

func (r *OpenAIRouter) resolveTenantPolicy(tenantID string) (config.ResolvedTenantPolicy, *services.TenantPolicyEvaluation) {
	policyConfig := config.TenantPolicyConfig{}
	if r != nil && r.Config != nil {
		policyConfig = r.Config.TenantPolicy
	}
	resolved := policyConfig.Resolve(strings.TrimSpace(tenantID))
	evidence := &services.TenantPolicyEvaluation{
		ContractVersion:  config.TenantPolicyContractVersion,
		PolicyVersion:    policyConfig.Version,
		Status:           "disabled",
		Source:           resolved.Source,
		TenantPresent:    resolved.TenantPresent,
		RequireTenant:    policyConfig.RequireTenant,
		AllowedModels:    append([]string(nil), resolved.Policy.AllowedModels...),
		DeniedModels:     append([]string(nil), resolved.Policy.DeniedModels...),
		AllowedProviders: append([]string(nil), resolved.Policy.AllowedProviders...),
		DeniedProviders:  append([]string(nil), resolved.Policy.DeniedProviders...),
		MaxEstimatedCost: cloneFloat64(resolved.Policy.MaxEstimatedCost),
	}
	if resolved.Enabled {
		evidence.Status = "applied"
		if policyConfig.RequireTenant && !resolved.TenantPresent {
			evidence.Status = "tenant_required"
		}
	}
	return resolved, evidence
}

func (r *OpenAIRouter) tenantPolicyEligibleModelRefs(
	refs []config.ModelRef,
	resolved config.ResolvedTenantPolicy,
) modelEligibilityResult {
	result := modelEligibilityResult{eligible: make([]config.ModelRef, 0, len(refs))}
	if !resolved.Enabled {
		result.eligible = append(result.eligible, refs...)
		return result
	}
	if r.Config.TenantPolicy.RequireTenant && !resolved.TenantPresent {
		for _, ref := range refs {
			result.exclusions = append(result.exclusions, services.ModelEligibilityExclusion{
				Model: ref.Model, Reasons: []string{"tenant_required"},
			})
		}
		return result
	}
	for _, ref := range refs {
		reasons := r.tenantPolicyExclusionReasons(ref.Model, resolved.Policy)
		if len(reasons) > 0 {
			result.exclusions = append(result.exclusions, services.ModelEligibilityExclusion{
				Model: ref.Model, Reasons: reasons,
			})
			continue
		}
		result.eligible = append(result.eligible, ref)
	}
	return result
}

func (r *OpenAIRouter) tenantPolicyExclusionReasons(model string, policy config.TenantRoutingPolicy) []string {
	model = strings.TrimSpace(model)
	provider := r.Config.ModelProvider(model)
	reasons := make([]string, 0, 2)
	if len(policy.AllowedModels) > 0 && !containsFold(policy.AllowedModels, model) {
		reasons = append(reasons, "tenant_model_not_allowed")
	}
	if containsFold(policy.DeniedModels, model) {
		reasons = append(reasons, "tenant_model_denied")
	}
	if len(policy.AllowedProviders) > 0 && !containsFold(policy.AllowedProviders, provider) {
		reasons = append(reasons, "tenant_provider_not_allowed")
	}
	if containsFold(policy.DeniedProviders, provider) {
		reasons = append(reasons, "tenant_provider_denied")
	}
	return reasons
}

func effectiveTenantRequestBudget(
	base *config.RequestBudget,
	policyConfig config.TenantPolicyConfig,
	policy config.TenantRoutingPolicy,
) *config.RequestBudget {
	if policy.MaxEstimatedCost == nil {
		return base
	}
	if base == nil {
		return &config.RequestBudget{
			Currency:              strings.ToUpper(strings.TrimSpace(policyConfig.Currency)),
			MaxEstimatedCost:      *policy.MaxEstimatedCost,
			OutputTokenBound:      policyConfig.OutputTokenBound,
			ReasoningTokenBound:   policyConfig.ReasoningTokenBound,
			RequirePricing:        policyConfig.RequirePricing,
			RequireCurrentPricing: policyConfig.RequireCurrentPricing,
		}
	}
	result := *base
	if policy.MaxEstimatedCost != nil &&
		(result.MaxEstimatedCost <= 0 || *policy.MaxEstimatedCost < result.MaxEstimatedCost) {
		result.MaxEstimatedCost = *policy.MaxEstimatedCost
	}
	return &result
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func (r *OpenAIRouter) tenantPolicyIneligibleAlgorithmModelCount(
	decision *config.Decision,
	resolved config.ResolvedTenantPolicy,
) int {
	if decision == nil || decision.Algorithm == nil {
		return 0
	}
	refs := make([]config.ModelRef, 0)
	seen := make(map[string]struct{})
	for _, model := range explicitAlgorithmModels(decision.Algorithm) {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		refs = append(refs, config.ModelRef{Model: model})
	}
	return len(r.tenantPolicyEligibleModelRefs(refs, resolved).exclusions)
}
