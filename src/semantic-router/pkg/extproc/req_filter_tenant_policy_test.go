package extproc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/services"
)

func TestSelectModelForEvalAppliesTenantProviderPolicy(t *testing.T) {
	router := tenantPolicyTestRouter()
	decision := &config.Decision{
		Name: "route",
		ModelRefs: []config.ModelRef{
			{Model: "cheap"},
			{Model: "premium"},
		},
	}

	selection := router.SelectModelForEval(services.EvalModelSelectionInput{
		Decision: decision,
		TenantID: "tenant-a",
	})

	require.NotNil(t, selection.TenantPolicy)
	assert.Equal(t, "applied", selection.TenantPolicy.Status)
	assert.Equal(t, "tenant", selection.TenantPolicy.Source)
	assert.True(t, selection.TenantPolicy.TenantPresent)
	assert.Equal(t, []string{"premium"}, selection.Eligibility.EligibleModels)
	require.Len(t, selection.Eligibility.ExcludedModels, 1)
	assert.Equal(t, []string{"tenant_provider_not_allowed"}, selection.Eligibility.ExcludedModels[0].Reasons)
}

func TestRuntimeTenantPolicyFailsClosedWithoutRequiredTenant(t *testing.T) {
	router := tenantPolicyTestRouter()
	router.Config.TenantPolicy.RequireTenant = true
	ctx := &RequestContext{Headers: map[string]string{}}

	_, err := router.contextEligibleDecisionModelRefs(
		[]config.ModelRef{{Model: "cheap"}}, "route", nil, 1, ctx,
	)

	require.Error(t, err)
	require.NotNil(t, ctx.VSRTenantPolicy)
	assert.Equal(t, "tenant_required", ctx.VSRTenantPolicy.Status)
}

func TestExplicitModelCannotBypassTenantPolicy(t *testing.T) {
	router := tenantPolicyTestRouter()
	ctx := &RequestContext{Headers: map[string]string{"x-authz-tenant-id": "tenant-a"}}

	err := router.validateTenantPolicyModel("cheap", nil, ctx)

	require.Error(t, err)
	assert.ErrorContains(t, err, "tenant-policy eligibility")
	require.NotNil(t, ctx.VSRTenantPolicy)
	assert.Equal(t, "tenant", ctx.VSRTenantPolicy.Source)
}

func TestEffectiveTenantRequestBudgetUsesStricterCap(t *testing.T) {
	decisionCap := 0.08
	tenantCap := 0.01
	result := effectiveTenantRequestBudget(
		&config.RequestBudget{MaxEstimatedCost: decisionCap},
		config.TenantPolicyConfig{},
		config.TenantRoutingPolicy{MaxEstimatedCost: &tenantCap},
	)
	require.NotNil(t, result)
	assert.Equal(t, tenantCap, result.MaxEstimatedCost)
	assert.Equal(t, decisionCap, (&config.RequestBudget{MaxEstimatedCost: decisionCap}).MaxEstimatedCost)
}

func TestEffectiveTenantRequestBudgetCreatesPolicyBudget(t *testing.T) {
	tenantCap := 0.01
	result := effectiveTenantRequestBudget(
		nil,
		config.TenantPolicyConfig{
			Currency: "USD", OutputTokenBound: 512, ReasoningTokenBound: 1024,
			RequirePricing: true, RequireCurrentPricing: true,
		},
		config.TenantRoutingPolicy{MaxEstimatedCost: &tenantCap},
	)

	require.NotNil(t, result)
	assert.Equal(t, "USD", result.Currency)
	assert.Equal(t, tenantCap, result.MaxEstimatedCost)
	assert.Equal(t, 512, result.OutputTokenBound)
	assert.True(t, result.RequirePricing)
	assert.True(t, result.RequireCurrentPricing)
}

func tenantPolicyTestRouter() *OpenAIRouter {
	return &OpenAIRouter{Config: &config.RouterConfig{
		BackendModels: config.BackendModels{ModelConfig: map[string]config.ModelParams{
			"cheap":   {Tags: []string{"provider:mistral"}},
			"premium": {Tags: []string{"provider:openai"}},
		}},
		TenantPolicy: config.TenantPolicyConfig{
			Enabled: true,
			Version: "policy-v1",
			Tenants: map[string]config.TenantRoutingPolicy{
				"tenant-a": {AllowedProviders: []string{"openai"}},
			},
		},
	}}
}
