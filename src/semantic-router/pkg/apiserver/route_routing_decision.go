//go:build !windows && cgo

package apiserver

import (
	"net/http"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/classification"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/decision"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/services"
)

const (
	routingDecisionSchemaVersion       = "vllm-sr/routing-decision/v1alpha1"
	stableRoutingDecisionSchemaVersion = "vllm-sr/routing-decision/v1"
)

// RoutingDecisionEnvelope is the privacy-minimized, non-generating routing
// preview returned to products integrating with the Router.
type RoutingDecisionEnvelope struct {
	SchemaVersion string                           `json:"schema_version"`
	DryRun        bool                             `json:"dry_run"`
	Route         RoutingDecisionRoute             `json:"route"`
	Selection     RoutingDecisionSelection         `json:"selection"`
	Eligibility   *services.ModelEligibility       `json:"eligibility,omitempty"`
	Cost          *services.RequestCostEvaluation  `json:"cost,omitempty"`
	Workflow      *services.WorkflowEvaluation     `json:"workflow,omitempty"`
	Resilience    *services.ResilienceEvaluation   `json:"resilience,omitempty"`
	TenantPolicy  *services.TenantPolicyEvaluation `json:"tenant_policy,omitempty"`
	Signals       RoutingDecisionSignals           `json:"signals"`
	Diagnostics   RoutingDecisionDiagnostics       `json:"diagnostics,omitempty"`
	Trace         []decision.DecisionTrace         `json:"trace,omitempty"`
}

type RoutingDecisionRoute struct {
	Recipe    config.RecipeName `json:"recipe,omitempty"`
	Decision  string            `json:"decision,omitempty"`
	Algorithm string            `json:"algorithm,omitempty"`
}

type RoutingDecisionSelection struct {
	Status          string   `json:"status,omitempty"`
	Method          string   `json:"method,omitempty"`
	SelectedModel   string   `json:"selected_model,omitempty"`
	CandidateModels []string `json:"candidate_models,omitempty"`
	Reason          string   `json:"reason,omitempty"`
}

type RoutingDecisionSignals struct {
	Used        *services.MatchedSignals                `json:"used,omitempty"`
	Matched     *services.MatchedSignals                `json:"matched,omitempty"`
	Unmatched   *services.MatchedSignals                `json:"unmatched,omitempty"`
	Confidences map[string]float64                      `json:"confidences,omitempty"`
	Values      map[string]float64                      `json:"values,omitempty"`
	Metrics     *classification.SignalMetricsCollection `json:"metrics,omitempty"`
}

type RoutingDecisionDiagnostics struct {
	SignalErrors           map[string]string `json:"signal_errors,omitempty"`
	AppliedUnknownPolicies map[string]string `json:"applied_unknown_policies,omitempty"`
	DecisionError          string            `json:"decision_error,omitempty"`
}

// handleRoutingDecision evaluates the live routing policy without forwarding
// the request to a model or tool. The response deliberately excludes raw text,
// messages, tools, and request metadata.
func (s *ClassificationAPIServer) handleRoutingDecision(w http.ResponseWriter, r *http.Request) {
	var req services.IntentRequest
	if err := s.parseJSONRequest(r, &req); err != nil {
		s.writeJSONRequestError(w, err)
		return
	}

	if req.Options == nil {
		req.Options = &services.IntentOptions{}
	}
	req.Options.EvaluateAllSignals = true
	if r.URL.Query().Get("trace") == "true" {
		req.Options.Trace = true
	}
	if cfg := s.currentConfig(); cfg != nil && cfg.TenantPolicy.Enabled {
		req.TenantID = r.Header.Get(cfg.TenantPolicy.GetTenantIDHeader())
	}

	evaluated, err := s.classificationSvc.ClassifyIntentForEval(req)
	if evaluated != nil {
		status := http.StatusOK
		if err != nil {
			status = http.StatusServiceUnavailable
		}
		s.writeJSONResponse(w, status, newRoutingDecisionEnvelope(evaluated, routingDecisionSchemaForPath(r.URL.Path)))
		return
	}
	if err != nil {
		s.writeClassificationError(w, err)
		return
	}

	s.writeErrorResponse(
		w,
		http.StatusInternalServerError,
		"ROUTING_DECISION_UNAVAILABLE",
		"routing decision evaluation returned no result",
	)
}

func newRoutingDecisionEnvelope(evaluated *services.EvalResponse, schemaVersion string) RoutingDecisionEnvelope {
	envelope := RoutingDecisionEnvelope{
		SchemaVersion: schemaVersion,
		DryRun:        true,
		Route: RoutingDecisionRoute{
			Recipe: evaluated.Recipe,
		},
		Selection: RoutingDecisionSelection{
			Status:          evaluated.SelectionStatus,
			Method:          evaluated.SelectionMethod,
			SelectedModel:   evaluated.SelectedModel,
			CandidateModels: append([]string(nil), evaluated.RecommendedModels...),
			Reason:          evaluated.SelectionReason,
		},
		Eligibility:  evaluated.Eligibility,
		Cost:         evaluated.Cost,
		Workflow:     evaluated.Workflow,
		Resilience:   evaluated.Resilience,
		TenantPolicy: evaluated.TenantPolicy,
		Signals: RoutingDecisionSignals{
			Confidences: evaluated.SignalConfidences,
			Values:      evaluated.SignalValues,
			Metrics:     evaluated.Metrics,
		},
		Diagnostics: RoutingDecisionDiagnostics{
			SignalErrors:           evaluated.SignalErrors,
			AppliedUnknownPolicies: evaluated.AppliedUnknownPolicies,
			DecisionError:          evaluated.DecisionError,
		},
		Trace: append([]decision.DecisionTrace(nil), evaluated.EvalTrace...),
	}
	if evaluated.DecisionResult != nil {
		envelope.Route.Decision = evaluated.DecisionResult.DecisionName
		envelope.Route.Algorithm = evaluated.DecisionResult.Algorithm
		envelope.Signals.Used = evaluated.DecisionResult.UsedSignals
		envelope.Signals.Matched = evaluated.DecisionResult.MatchedSignals
		envelope.Signals.Unmatched = evaluated.DecisionResult.UnmatchedSignals
	}
	return envelope
}

func routingDecisionSchemaForPath(path string) string {
	if path == "/v1/route/evaluate" {
		return stableRoutingDecisionSchemaVersion
	}
	return routingDecisionSchemaVersion
}
