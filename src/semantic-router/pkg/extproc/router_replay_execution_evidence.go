package extproc

import (
	"strconv"
	"time"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/observability/logging"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/observability/metrics"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/routerreplay"
)

const (
	executionEvidenceContractVersion = "vllm-sr/execution-evidence/v1alpha1"
	executionEvidenceOutcomeSource   = "router_execution"
)

func (r *OpenAIRouter) recordExecutionEvidence(
	ctx *RequestContext,
	latency time.Duration,
) {
	if ctx == nil || ctx.RouterReplayID == "" || ctx.VSRExecutionEvidenceRecorded {
		return
	}
	recorder := ctx.RouterReplayRecorder
	if recorder == nil {
		recorder = r.ReplayRecorder
	}
	if recorder == nil {
		return
	}
	record, ok := recorder.GetRecord(ctx.RouterReplayID)
	if !ok {
		return
	}
	usage := replayRecordUsage(record)

	algorithm := ctx.VSRSelectionMethod
	modelAttempts := max(ctx.VSRModelAttempts, 1)
	providerAttempts := max(ctx.VSRProviderAttempts, modelAttempts)
	fallbackRetries := 0
	if algorithm == config.DecisionAlgorithmFallback {
		fallbackRetries = max(modelAttempts-1, 0)
	}
	providerRetries := max(providerAttempts-modelAttempts, 0)
	qualityStatus, qualityScore := executionQualityEvidence(ctx)
	metadata := map[string]string{
		"contract_version":            executionEvidenceContractVersion,
		"capability_catalog_version":  config.ModelCapabilityCatalogVersion,
		"pricing_catalog_version":     config.ProviderPricingCatalogVersion,
		"resilience_contract_version": config.ResilienceContractVersion,
		"classifier_contract_version": config.LocalSemanticClassifierContractVersion,
		"algorithm":                   algorithm,
		"model_attempts":              strconv.Itoa(modelAttempts),
		"provider_attempts":           strconv.Itoa(providerAttempts),
		"provider_retries":            strconv.Itoa(providerRetries),
		"fallback_retries":            strconv.Itoa(fallbackRetries),
		"fallback_used":               strconv.FormatBool(fallbackRetries > 0),
		"latency_ms":                  strconv.FormatInt(latency.Milliseconds(), 10),
		"quality_status":              qualityStatus,
	}
	if r.Config != nil {
		if pricing, ok := r.Config.GetFullModelPricing(ctx.VSRSelectedModel); ok {
			metadata["price_version"] = pricing.Version
			metadata["price_source"] = pricing.Source
		}
	}
	appendUsageEvidenceMetadata(metadata, usage)
	outcome := routerreplay.Outcome{
		Timestamp: time.Now().UTC(), Source: executionEvidenceOutcomeSource,
		Target: "response", TargetRef: ctx.VSRSelectedModel,
		Verdict: qualityStatus, Score: qualityScore, Metadata: metadata,
	}
	if err := recorder.AppendOutcome(ctx.RouterReplayID, outcome); err != nil {
		logging.ComponentErrorEvent("extproc", "router_replay_execution_evidence_failed", map[string]interface{}{
			"request_id": ctx.RequestID, "replay_id": ctx.RouterReplayID, "error": err.Error(),
		})
		return
	}
	ctx.VSRExecutionEvidenceRecorded = true
	metrics.RecordExecutionEvidence(
		algorithm, fallbackRetries > 0, qualityStatus, providerRetries,
		fallbackRetries, replayStringValue(usage.Currency), replayFloatValue(usage.CostSavings),
	)
}

func replayRecordUsage(record routerreplay.RoutingRecord) routerreplay.UsageCost {
	return routerreplay.UsageCost{
		PromptTokens: record.PromptTokens, CachedPromptTokens: record.CachedPromptTokens,
		CacheWriteTokens: record.CacheWriteTokens, CompletionTokens: record.CompletionTokens,
		TotalTokens: record.TotalTokens, ActualCost: record.ActualCost,
		BaselineCost: record.BaselineCost, CostSavings: record.CostSavings,
		Currency: record.Currency, BaselineModel: record.BaselineModel,
	}
}

func executionQualityEvidence(ctx *RequestContext) (string, float64) {
	if ctx == nil || ctx.VSRSelectedDecision == nil ||
		ctx.VSRSelectedDecision.GetHallucinationConfig() == nil ||
		!ctx.VSRSelectedDecision.GetHallucinationConfig().Enabled {
		return "not_measured", 0
	}
	if ctx.HallucinationDetected || ctx.UnverifiedFactualResponse {
		return "failed", float64(1 - ctx.HallucinationConfidence)
	}
	return "passed", float64(1 - ctx.HallucinationConfidence)
}

func appendUsageEvidenceMetadata(metadata map[string]string, usage routerreplay.UsageCost) {
	if usage.TotalTokens != nil {
		metadata["total_tokens"] = strconv.Itoa(*usage.TotalTokens)
	}
	if usage.ActualCost != nil {
		metadata["actual_cost"] = strconv.FormatFloat(*usage.ActualCost, 'g', -1, 64)
	}
	if usage.BaselineCost != nil {
		metadata["baseline_cost"] = strconv.FormatFloat(*usage.BaselineCost, 'g', -1, 64)
	}
	if usage.CostSavings != nil {
		metadata["cost_savings"] = strconv.FormatFloat(*usage.CostSavings, 'g', -1, 64)
	}
	if usage.Currency != nil {
		metadata["currency"] = *usage.Currency
	}
	if usage.BaselineModel != nil {
		metadata["baseline_model"] = *usage.BaselineModel
	}
}

func replayStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func replayFloatValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
