package extproc

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/routerreplay"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/routerreplay/store"
)

func TestRecordExecutionEvidenceIsVersionedBoundedAndContentFree(t *testing.T) {
	recorder := routerreplay.NewRecorder(store.NewMemoryStore(10, 0))
	replayID, err := recorder.AddRecord(routerreplay.RoutingRecord{
		ID: "evidence-1", LifecycleState: routerreplay.LifecycleInProgress,
	})
	if err != nil {
		t.Fatalf("add replay record: %v", err)
	}
	router := &OpenAIRouter{ReplayRecorder: recorder, Config: executionEvidencePricingConfig()}
	ctx := &RequestContext{
		RequestID: "request-1", RequestQuery: "private prompt must not appear",
		RouterReplayID: replayID, RouterReplayRecorder: recorder,
		VSRSelectedModel: "cheap", VSRSelectionMethod: config.DecisionAlgorithmFallback,
		VSRModelAttempts: 2, VSRProviderAttempts: 3,
	}
	usage := router.buildReplayUsageCost(ctx, responseUsageMetrics{
		promptTokens: 100, promptTokensReported: true,
		completionTokens: 20, completionTokensReported: true,
		totalTokens: 120, totalTokensReported: true,
	})
	router.updateRouterReplayUsageCost(ctx, usage)
	router.recordExecutionEvidence(ctx, 125*time.Millisecond)
	router.recordExecutionEvidence(ctx, 250*time.Millisecond)

	record, ok := recorder.GetRecord(replayID)
	if !ok || len(record.Outcomes) != 1 {
		t.Fatalf("execution outcomes = %#v", record.Outcomes)
	}
	metadata := record.Outcomes[0].Metadata
	assertExecutionEvidenceMetadata(t, metadata, map[string]string{
		"contract_version": executionEvidenceContractVersion,
		"model_attempts":   "2", "provider_attempts": "3",
		"provider_retries": "1", "fallback_retries": "1",
		"latency_ms": "125", "quality_status": "not_measured",
	})
	encoded, err := json.Marshal(record.Outcomes[0])
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}
	if strings.Contains(string(encoded), ctx.RequestQuery) {
		t.Fatal("execution evidence exposed raw request content")
	}
}

func assertExecutionEvidenceMetadata(t *testing.T, actual, expected map[string]string) {
	t.Helper()
	for key, expectedValue := range expected {
		if actual[key] != expectedValue {
			t.Fatalf("execution evidence %q = %q, want %q; metadata=%#v", key, actual[key], expectedValue, actual)
		}
	}
}

func executionEvidencePricingConfig() *config.RouterConfig {
	return &config.RouterConfig{BackendModels: config.BackendModels{ModelConfig: map[string]config.ModelParams{
		"cheap": {Pricing: config.ModelPricing{
			Version: "price-v1", Source: "test", Currency: "USD",
			PromptPer1M: 1, CompletionPer1M: 2,
		}},
		"premium": {Pricing: config.ModelPricing{
			Version: "price-v1", Source: "test", Currency: "USD",
			PromptPer1M: 10, CompletionPer1M: 20,
		}},
	}}}
}

func TestBuildRouterReplayExecutionEvidenceSummary(t *testing.T) {
	records := []routerreplay.RoutingRecord{{Outcomes: []routerreplay.Outcome{{
		Source: executionEvidenceOutcomeSource,
		Metadata: map[string]string{
			"contract_version": executionEvidenceContractVersion,
			"model_attempts":   "2", "provider_attempts": "3",
			"provider_retries": "1", "fallback_retries": "1",
			"fallback_used": "true", "latency_ms": "125", "quality_status": "passed",
		},
	}}}}
	summary := buildRouterReplayExecutionEvidence(records)
	if summary.RecordCount != 1 || summary.ModelAttempts != 2 ||
		summary.ProviderAttempts != 3 || summary.ProviderRetries != 1 ||
		summary.FallbackRetries != 1 || summary.FallbackRequestCount != 1 ||
		summary.QualityPassed != 1 || summary.TotalLatencyMS != 125 {
		t.Fatalf("unexpected execution evidence summary: %+v", summary)
	}
}

func TestExecutionEvidenceDoesNotCallNonFallbackIterationsFallback(t *testing.T) {
	recorder := routerreplay.NewRecorder(store.NewMemoryStore(10, 0))
	replayID, err := recorder.AddRecord(routerreplay.RoutingRecord{
		ID: "confidence-evidence", LifecycleState: routerreplay.LifecycleInProgress,
	})
	if err != nil {
		t.Fatalf("add replay record: %v", err)
	}
	router := &OpenAIRouter{ReplayRecorder: recorder}
	ctx := &RequestContext{
		RouterReplayID: replayID, RouterReplayRecorder: recorder,
		VSRSelectionMethod: "confidence", VSRSelectedModel: "candidate",
		VSRModelAttempts: 3,
	}
	router.recordExecutionEvidence(ctx, time.Millisecond)

	record, ok := recorder.GetRecord(replayID)
	if !ok || len(record.Outcomes) != 1 {
		t.Fatalf("execution outcomes = %#v", record.Outcomes)
	}
	metadata := record.Outcomes[0].Metadata
	if metadata["model_attempts"] != "3" || metadata["fallback_retries"] != "0" ||
		metadata["fallback_used"] != "false" {
		t.Fatalf("non-fallback evidence was mislabeled: %#v", metadata)
	}
}
