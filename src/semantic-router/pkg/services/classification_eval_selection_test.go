package services

import (
	"testing"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/classification"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/decision"
)

type evalModelSelectorStub struct {
	input EvalModelSelectionInput
}

func (s *evalModelSelectorStub) SelectModelForEval(
	input EvalModelSelectionInput,
) EvalModelSelection {
	s.input = input
	return EvalModelSelection{
		SelectedModel: "model-b",
		Status:        EvalSelectionSelected,
		Method:        "multi_factor",
		Reason:        "highest live score",
	}
}

func TestPopulateEvalModelSelectionReturnsConcreteRuntimeChoice(t *testing.T) {
	selector := &evalModelSelectorStub{}
	service := &ClassificationService{}
	service.SetEvalModelSelector(selector)
	response := &EvalResponse{Recipe: "balanced"}
	matchedDecision := &config.Decision{
		Name:      "balanced-route",
		ModelRefs: []config.ModelRef{{Model: "model-a"}, {Model: "model-b"}},
		Workflow: &config.WorkflowConfig{
			Type: config.WorkflowWebSearchAnswer, AuthorizationGroup: "web-search-users",
			WebSearch: &config.WebSearchWorkflowConfig{
				Provider: config.WebSearchProviderSearXNG, Endpoint: "https://search.example.com/search",
				TimeoutSeconds: 5, MaxResults: 4, MaxQueryCharacters: 500,
				MaxResponseBytes: 262144, MaxEvidenceCharacters: 8000,
			},
		},
	}
	service.populateEvalModelSelection(
		response,
		intentSignalInput{
			currentUserText: "Explain the tradeoff.",
			inputTokenFloor: 3072,
			requestFacts: classification.RequestFacts{
				ContextTokenFloor: 4096,
			},
		},
		&decision.DecisionResult{
			Decision:     matchedDecision,
			MatchedRules: []string{"domain:engineering"},
		},
		0,
		0,
		"",
	)
	assertConcreteRuntimeChoice(t, selector, response, matchedDecision)
}

func TestWorkflowEvaluationDescribesMCPWithoutExecutingIt(t *testing.T) {
	evaluation := workflowEvaluation(&config.WorkflowConfig{
		Type: config.WorkflowMCPToolCall, AuthorizationGroup: "mcp-users",
		MCP: &config.MCPWorkflowConfig{ServerName: "catalog", ToolName: "lookup", TimeoutSeconds: 5,
			MaxResponseBytes: 262144, MaxResultCharacters: 8000, RequireReadOnly: true},
	}, "synth")
	if evaluation == nil || evaluation.ExecutesTools || evaluation.Tool.Type != "mcp" ||
		evaluation.Tool.ToolName != "lookup" || !evaluation.Tool.RequiresReadOnly {
		t.Fatalf("MCP workflow evaluation = %+v", evaluation)
	}
}

func assertConcreteRuntimeChoice(
	t *testing.T,
	selector *evalModelSelectorStub,
	response *EvalResponse,
	matchedDecision *config.Decision,
) {
	t.Helper()
	assertEvalSelection(t, response)
	assertEvalSelectorInput(t, selector, matchedDecision)
	assertEvalWorkflow(t, response)
}

func assertEvalSelection(t *testing.T, response *EvalResponse) {
	t.Helper()
	if response.SelectedModel != "model-b" || response.SelectionStatus != EvalSelectionSelected {
		t.Fatalf("selection response = %+v", response)
	}
}

func assertEvalSelectorInput(t *testing.T, selector *evalModelSelectorStub, matchedDecision *config.Decision) {
	t.Helper()
	if selector.input.Decision != matchedDecision || selector.input.Recipe != "balanced" {
		t.Fatalf("selector scope = %+v", selector.input)
	}
	if selector.input.Query != "Explain the tradeoff." || selector.input.Category != "engineering" {
		t.Fatalf("selector semantic input = %+v", selector.input)
	}
	if selector.input.ContextTokenCount != 4096 {
		t.Fatalf("selector context count = %d", selector.input.ContextTokenCount)
	}
	if selector.input.InputTokenCount != 3072 {
		t.Fatalf("selector input token count = %d", selector.input.InputTokenCount)
	}
}

func assertEvalWorkflow(t *testing.T, response *EvalResponse) {
	t.Helper()
	if response.Workflow == nil || response.Workflow.Status != "planned" || response.Workflow.ExecutesTools {
		t.Fatalf("dry-run workflow = %+v", response.Workflow)
	}
	if response.Workflow.Authorization.Status != "not_evaluated" || response.Workflow.SynthesisModel != "model-b" {
		t.Fatalf("dry-run workflow evidence = %+v", response.Workflow)
	}
}

func TestPopulateEvalModelSelectionDoesNotInventFirstRecommendedModel(t *testing.T) {
	response := &EvalResponse{Recipe: "accuracy"}
	service := &ClassificationService{}
	service.populateEvalModelSelection(
		response,
		intentSignalInput{},
		&decision.DecisionResult{Decision: &config.Decision{
			Name:      "fusion-route",
			ModelRefs: []config.ModelRef{{Model: "model-a"}, {Model: "model-b"}},
		}},
		0,
		0,
		"",
	)

	if response.SelectedModel != "" || response.SelectionStatus != EvalSelectionUnavailable {
		t.Fatalf("unwired Eval invented a final model: %+v", response)
	}
}
