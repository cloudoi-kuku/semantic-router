//go:build !windows && cgo

package apiserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/decision"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/services"
)

func TestHandleRoutingDecisionReturnsPrivacyMinimizedDecision(t *testing.T) {
	const privatePrompt = "PRIVATE_ROUTING_PROMPT"
	fakeSvc := &evalCaptureClassificationService{evalResp: &services.EvalResponse{
		OriginalText:      privatePrompt,
		RequestedModel:    "niffy/auto",
		Recipe:            config.RecipeName("default"),
		RecommendedModels: []string{"niffy-reasoning"},
		SelectedModel:     "niffy-reasoning",
		SelectionStatus:   services.EvalSelectionSelected,
		SelectionMethod:   "single",
		SelectionReason:   "one eligible model",
		Eligibility: &services.ModelEligibility{
			CatalogVersion:       config.ModelCapabilityCatalogVersion,
			RequiredCapabilities: []string{"chat", "reasoning"},
			EligibleModels:       []string{"niffy-reasoning"},
		},
		Cost: &services.RequestCostEvaluation{
			CatalogVersion: config.ProviderPricingCatalogVersion,
			Currency:       "USD", InputTokens: 100, OutputTokens: 1000, MaxCost: 0.12,
			Candidates: []services.CandidateCostEstimate{{Model: "niffy-reasoning", EstimatedCost: 0.0122, Eligible: true, Status: "within_budget"}},
		},
		Workflow: &services.WorkflowEvaluation{
			ContractVersion: config.WorkflowContractVersion, Type: config.WorkflowWebSearchAnswer,
			Status: "planned", ExecutesTools: false,
			Authorization:  services.WorkflowAuthorizationEvidence{RequiredGroup: "web-search-users", Status: "not_evaluated"},
			Tool:           services.WorkflowToolPlan{Type: "web_search", Provider: "searxng", MaxResults: 5},
			SynthesisModel: "niffy-reasoning",
		},
		Resilience: &services.ResilienceEvaluation{
			ContractVersion: config.ResilienceContractVersion,
			Type:            "ordered_fallback", Status: "planned", MaxAttempts: 2,
			RetryOn: []string{"server_error"}, CandidateModels: []string{"niffy-reasoning"},
			ExecutesModels: false,
		},
		DecisionResult: &services.EvalDecisionResult{
			DecisionName: "reasoning-route",
			Algorithm:    "static",
			UsedSignals:  &services.MatchedSignals{Keywords: []string{"reasoning_intent"}},
			MatchedSignals: &services.MatchedSignals{
				Keywords: []string{"reasoning_intent"},
			},
			UnmatchedSignals: &services.MatchedSignals{},
		},
	}}
	server := &ClassificationAPIServer{classificationSvc: fakeSvc}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/route/evaluate?trace=true",
		strings.NewReader(`{"model":"niffy/auto","messages":[{"role":"user","content":"PRIVATE_ROUTING_PROMPT"}]}`),
	)
	recorder := httptest.NewRecorder()
	server.handleRoutingDecision(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), privatePrompt) ||
		strings.Contains(recorder.Body.String(), "original_text") {
		t.Fatalf("response exposed private request content: %s", recorder.Body.String())
	}
	var response RoutingDecisionEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	assertRoutingDecisionResponse(t, response)
	assertRoutingDecisionOptions(t, fakeSvc)
}

func assertRoutingDecisionResponse(t *testing.T, response RoutingDecisionEnvelope) {
	t.Helper()
	if response.SchemaVersion != routingDecisionSchemaVersion || !response.DryRun {
		t.Fatalf("unexpected contract header: %+v", response)
	}
	if response.Route.Decision != "reasoning-route" ||
		response.Selection.SelectedModel != "niffy-reasoning" {
		t.Fatalf("unexpected routing decision: %+v", response)
	}
	assertRoutingDecisionEligibility(t, response)
	assertRoutingDecisionCost(t, response)
	if response.Workflow == nil || response.Workflow.ContractVersion != config.WorkflowContractVersion || response.Workflow.ExecutesTools {
		t.Fatalf("missing or unsafe workflow preview: %+v", response.Workflow)
	}
	if response.Resilience == nil || response.Resilience.ContractVersion != config.ResilienceContractVersion || response.Resilience.ExecutesModels {
		t.Fatalf("missing or unsafe resilience preview: %+v", response.Resilience)
	}
}

func assertRoutingDecisionOptions(t *testing.T, fakeSvc *evalCaptureClassificationService) {
	t.Helper()
	if fakeSvc.lastEvalReq.Options == nil ||
		!fakeSvc.lastEvalReq.Options.EvaluateAllSignals ||
		!fakeSvc.lastEvalReq.Options.Trace {
		t.Fatalf("routing options were not forwarded: %+v", fakeSvc.lastEvalReq.Options)
	}
}

func assertRoutingDecisionCost(t *testing.T, response RoutingDecisionEnvelope) {
	t.Helper()
	if response.Cost == nil || response.Cost.CatalogVersion != config.ProviderPricingCatalogVersion || len(response.Cost.Candidates) != 1 {
		t.Fatalf("missing request cost evidence: %+v", response)
	}
}

func assertRoutingDecisionEligibility(t *testing.T, response RoutingDecisionEnvelope) {
	t.Helper()
	if response.Eligibility == nil ||
		response.Eligibility.CatalogVersion != config.ModelCapabilityCatalogVersion {
		t.Fatalf("missing model eligibility evidence: %+v", response)
	}
}

func TestHandleRoutingDecisionReturnsDecisionDiagnosticsOnUnresolvedRoute(t *testing.T) {
	fakeSvc := &evalCaptureClassificationService{
		evalResp: &services.EvalResponse{
			DecisionError: "decision unresolved",
			EvalTrace:     []decision.DecisionTrace{{DecisionName: "guarded", State: "unknown"}},
		},
		evalErr: decision.ErrDecisionUnresolved,
	}
	server := &ClassificationAPIServer{classificationSvc: fakeSvc}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/route/evaluate?trace=true",
		strings.NewReader(`{"text":"hello"}`),
	)
	recorder := httptest.NewRecorder()

	server.handleRoutingDecision(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response RoutingDecisionEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Diagnostics.DecisionError == "" || len(response.Trace) != 1 {
		t.Fatalf("missing decision diagnostics: %+v", response)
	}
}
