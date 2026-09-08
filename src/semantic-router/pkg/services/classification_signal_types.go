package services

import (
	"encoding/json"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/classification"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/decision"
)

const (
	EvalSelectionSelected          = "selected"
	EvalSelectionPlannedFinal      = "planned_final"
	EvalSelectionFallback          = "fallback"
	EvalSelectionExecutionRequired = "execution_required"
	EvalSelectionUnavailable       = "unavailable"
	EvalSelectionFailed            = "failed"
)

// IntentRequest represents a request for intent classification.
type IntentRequest struct {
	Text                  string            `json:"text"`
	Messages              []IntentMessage   `json:"messages,omitempty"`
	Tools                 []json.RawMessage `json:"tools,omitempty"`
	Functions             []json.RawMessage `json:"functions,omitempty"`
	ToolChoice            json.RawMessage   `json:"tool_choice,omitempty"`
	FunctionCall          json.RawMessage   `json:"function_call,omitempty"`
	ResponseFormat        json.RawMessage   `json:"response_format,omitempty"`
	MaxTokens             json.RawMessage   `json:"max_tokens,omitempty"`
	MaxCompletionTokens   json.RawMessage   `json:"max_completion_tokens,omitempty"`
	ReasoningBudgetTokens json.RawMessage   `json:"reasoning_budget_tokens,omitempty"`
	Model                 string            `json:"model,omitempty"`
	Metadata              map[string]string `json:"metadata,omitempty"`
	Options               *IntentOptions    `json:"options,omitempty"`
	// TenantID is populated only from a trusted request header by the product
	// routing endpoint. It is never decoded from JSON or returned to callers.
	TenantID string `json:"-"`
}

// IntentOptions contains options for intent classification.
type IntentOptions struct {
	ReturnProbabilities bool    `json:"return_probabilities,omitempty"`
	ConfidenceThreshold float64 `json:"confidence_threshold,omitempty"`
	IncludeExplanation  bool    `json:"include_explanation,omitempty"`
	EvaluateAllSignals  bool    `json:"evaluate_all_signals,omitempty"` // Force evaluate all configured signals (for eval scenarios)
	Trace               bool    `json:"trace,omitempty"`                // Return per-decision evaluation trace trees
}

// MatchedSignals represents all matched signals from signal evaluation.
type MatchedSignals struct {
	Keywords      []string `json:"keywords,omitempty"`
	Embeddings    []string `json:"embeddings,omitempty"`
	Domains       []string `json:"domains,omitempty"`
	FactCheck     []string `json:"fact_check,omitempty"`
	UserFeedback  []string `json:"user_feedback,omitempty"`
	Reask         []string `json:"reask,omitempty"`
	Preferences   []string `json:"preferences,omitempty"`
	Language      []string `json:"language,omitempty"`
	Context       []string `json:"context,omitempty"`
	Structure     []string `json:"structure,omitempty"`
	Complexity    []string `json:"complexity,omitempty"`
	Modality      []string `json:"modality,omitempty"`
	Authz         []string `json:"authz,omitempty"`
	Jailbreak     []string `json:"jailbreak,omitempty"`
	PII           []string `json:"pii,omitempty"`
	KB            []string `json:"kb,omitempty"`
	Conversation  []string `json:"conversation,omitempty"`
	Event         []string `json:"event,omitempty"`
	Metadata      []string `json:"metadata,omitempty"`
	Classifier    []string `json:"classifier,omitempty"`
	InputModality []string `json:"input_modality,omitempty"`
	Projection    []string `json:"projection,omitempty"`
}

// DecisionResult represents the result of decision evaluation.
type DecisionResult struct {
	DecisionName string   `json:"decision_name"`
	Confidence   float64  `json:"confidence"`
	MatchedRules []string `json:"matched_rules"`
}

// EvalDecisionResult represents the decision result for eval scenarios (without confidence).
type EvalDecisionResult struct {
	DecisionName     string          `json:"decision_name"`
	Algorithm        string          `json:"algorithm"`
	Plugins          []string        `json:"plugins,omitempty"`
	UsedSignals      *MatchedSignals `json:"used_signals"`      // Signals used by this decision (from decision rules)
	MatchedSignals   *MatchedSignals `json:"matched_signals"`   // Signals that matched
	UnmatchedSignals *MatchedSignals `json:"unmatched_signals"` // Signals that didn't match
}

// EvalResponse represents the eval classification response with comprehensive signal information.
type EvalResponse struct {
	OriginalText           string                                  `json:"original_text"` // The evaluated user turn or fallback query text
	RequestedModel         string                                  `json:"requested_model,omitempty"`
	Recipe                 config.RecipeName                       `json:"recipe,omitempty"`
	DecisionResult         *EvalDecisionResult                     `json:"decision_result,omitempty"`
	EvalTrace              []decision.DecisionTrace                `json:"eval_trace,omitempty"`         // Per-decision evaluation trace (when ?trace=true)
	RecommendedModels      []string                                `json:"recommended_models,omitempty"` // All models from matched decision's modelRefs
	SelectedModel          string                                  `json:"selected_model,omitempty"`     // Concrete selector result or configured final-output model
	SelectionStatus        string                                  `json:"selection_status,omitempty"`   // selected, planned_final, fallback, execution_required, unavailable, or failed
	SelectionMethod        string                                  `json:"selection_method,omitempty"`
	SelectionReason        string                                  `json:"selection_reason,omitempty"`
	Eligibility            *ModelEligibility                       `json:"eligibility,omitempty"`
	Cost                   *RequestCostEvaluation                  `json:"cost,omitempty"`
	Workflow               *WorkflowEvaluation                     `json:"workflow,omitempty"`
	Resilience             *ResilienceEvaluation                   `json:"resilience,omitempty"`
	TenantPolicy           *TenantPolicyEvaluation                 `json:"tenant_policy,omitempty"`
	RoutingDecision        string                                  `json:"routing_decision,omitempty"`
	Metrics                *classification.SignalMetricsCollection `json:"metrics"`                      // Performance and confidence for each signal
	SignalConfidences      map[string]float64                      `json:"signal_confidences,omitempty"` // Real ML confidence scores per signal, e.g. "domain:economics" -> 0.81
	SignalValues           map[string]float64                      `json:"signal_values,omitempty"`      // Raw signal values per signal when exposed, e.g. "structure:many_questions" -> 4
	SignalErrors           map[string]string                       `json:"signal_errors,omitempty"`
	AppliedUnknownPolicies map[string]string                       `json:"applied_unknown_policies,omitempty"`
	DecisionError          string                                  `json:"decision_error,omitempty"`
}

// ResilienceEvaluation is a non-executing preview of an ordered, bounded
// fallback chain. It never probes a provider during dry-run.
type ResilienceEvaluation struct {
	ContractVersion string   `json:"contract_version"`
	Type            string   `json:"type"`
	Status          string   `json:"status"`
	MaxAttempts     int      `json:"max_attempts"`
	RetryOn         []string `json:"retry_on"`
	CandidateModels []string `json:"candidate_models,omitempty"`
	ExecutesModels  bool     `json:"executes_models"`
}

// WorkflowEvaluation describes a planned workflow without executing its tool.
// Authorization is deliberately not inferred from prompt or API request data.
type WorkflowEvaluation struct {
	ContractVersion string                        `json:"contract_version"`
	Type            string                        `json:"type"`
	Status          string                        `json:"status"`
	Authorization   WorkflowAuthorizationEvidence `json:"authorization"`
	Tool            WorkflowToolPlan              `json:"tool"`
	SynthesisModel  string                        `json:"synthesis_model,omitempty"`
	ExecutesTools   bool                          `json:"executes_tools"`
}

type WorkflowAuthorizationEvidence struct {
	RequiredGroup string `json:"required_group"`
	Status        string `json:"status"`
}

type WorkflowToolPlan struct {
	Type                  string `json:"type"`
	Provider              string `json:"provider,omitempty"`
	ServerName            string `json:"server_name,omitempty"`
	ToolName              string `json:"tool_name,omitempty"`
	MaxResults            int    `json:"max_results,omitempty"`
	TimeoutSeconds        int    `json:"timeout_seconds"`
	MaxResponseBytes      int64  `json:"max_response_bytes"`
	MaxEvidenceCharacters int    `json:"max_evidence_characters,omitempty"`
	MaxResultCharacters   int    `json:"max_result_characters,omitempty"`
	RequiresReadOnly      bool   `json:"requires_read_only,omitempty"`
}

type ModelEligibility struct {
	CatalogVersion       string                      `json:"catalog_version"`
	RequiredCapabilities []string                    `json:"required_capabilities,omitempty"`
	EligibleModels       []string                    `json:"eligible_models,omitempty"`
	ExcludedModels       []ModelEligibilityExclusion `json:"excluded_models,omitempty"`
}

type ModelEligibilityExclusion struct {
	Model               string   `json:"model"`
	Reasons             []string `json:"reasons"`
	MissingCapabilities []string `json:"missing_capabilities,omitempty"`
}

// TenantPolicyEvaluation is privacy-safe evidence that tenant policy was
// considered. It intentionally excludes the tenant identifier and configured
// tenant-map keys.
type TenantPolicyEvaluation struct {
	ContractVersion  string   `json:"contract_version"`
	PolicyVersion    string   `json:"policy_version"`
	Status           string   `json:"status"`
	Source           string   `json:"source"`
	TenantPresent    bool     `json:"tenant_present"`
	RequireTenant    bool     `json:"require_tenant"`
	AllowedModels    []string `json:"allowed_models,omitempty"`
	DeniedModels     []string `json:"denied_models,omitempty"`
	AllowedProviders []string `json:"allowed_providers,omitempty"`
	DeniedProviders  []string `json:"denied_providers,omitempty"`
	MaxEstimatedCost *float64 `json:"max_estimated_cost,omitempty"`
}

// RequestCostEvaluation is a privacy-safe pre-execution estimate. It contains
// token bounds and configured prices only, never provider-reported actual cost.
type RequestCostEvaluation struct {
	CatalogVersion  string                  `json:"catalog_version"`
	Currency        string                  `json:"currency"`
	InputTokens     int                     `json:"input_tokens"`
	OutputTokens    int                     `json:"output_tokens"`
	ReasoningTokens int                     `json:"reasoning_tokens,omitempty"`
	MaxCost         float64                 `json:"max_cost"`
	Candidates      []CandidateCostEstimate `json:"candidates"`
}

type CandidateCostEstimate struct {
	Model                      string  `json:"model"`
	SingleAttemptEstimatedCost float64 `json:"single_attempt_estimated_cost,omitempty"`
	EstimatedCost              float64 `json:"estimated_cost,omitempty"`
	MaxProviderAttempts        int     `json:"max_provider_attempts,omitempty"`
	Eligible                   bool    `json:"eligible"`
	Status                     string  `json:"status"`
	PriceVersion               string  `json:"price_version,omitempty"`
	PriceSource                string  `json:"price_source,omitempty"`
	EffectiveAt                string  `json:"effective_at,omitempty"`
	ExpiresAt                  string  `json:"expires_at,omitempty"`
}

// EvalModelSelectionInput is the content-minimized selection contract passed
// from classification to the live Router selector. It intentionally excludes
// raw tool schemas and message bodies beyond the current semantic query.
type EvalModelSelectionInput struct {
	Recipe              config.RecipeName
	Decision            *config.Decision
	Query               string
	Category            string
	ContextTokenCount   int
	InputTokenCount     int
	OutputTokenBound    int
	ReasoningTokenBound int
	TenantID            string
}

type EvalModelSelection struct {
	SelectedModel string
	Status        string
	Method        string
	Reason        string
	Eligibility   *ModelEligibility
	Cost          *RequestCostEvaluation
	TenantPolicy  *TenantPolicyEvaluation
}

// EvalModelSelector performs a non-generating selection preview with the same
// runtime-owned selector registry used by data-plane routing.
type EvalModelSelector interface {
	SelectModelForEval(input EvalModelSelectionInput) EvalModelSelection
}

// IntentResponse represents the response from intent classification.
type IntentResponse struct {
	Classification   Classification     `json:"classification"`
	Probabilities    map[string]float64 `json:"probabilities,omitempty"`
	RecommendedModel string             `json:"recommended_model,omitempty"`
	RoutingDecision  string             `json:"routing_decision,omitempty"`

	// Signal-driven fields
	MatchedSignals         *MatchedSignals   `json:"matched_signals,omitempty"`
	DecisionResult         *DecisionResult   `json:"decision_result,omitempty"`
	SignalErrors           map[string]string `json:"signal_errors,omitempty"`
	AppliedUnknownPolicies map[string]string `json:"applied_unknown_policies,omitempty"`
}

// Classification represents basic classification result.
type Classification struct {
	Category         string  `json:"category"`
	Confidence       float64 `json:"confidence"`
	ProcessingTimeMs int64   `json:"processing_time_ms"`
}
