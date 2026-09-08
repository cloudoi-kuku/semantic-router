package config

import (
	"strings"
	"testing"
)

func TestValidateWorkflowContractsAcceptsBoundedSearXNGWorkflow(t *testing.T) {
	cfg := workflowValidationConfig()
	if err := validateWorkflowContracts(cfg); err != nil {
		t.Fatalf("validate workflow: %v", err)
	}
}

func TestValidateWorkflowContractsRejectsMissingAuthorization(t *testing.T) {
	cfg := workflowValidationConfig()
	cfg.Decisions[0].Workflow.AuthorizationGroup = ""
	if err := validateWorkflowContracts(cfg); err == nil || !strings.Contains(err.Error(), "authorization_group") {
		t.Fatalf("error = %v, want authorization_group failure", err)
	}
}

func TestValidateWorkflowContractsRejectsUnboundedSearch(t *testing.T) {
	cfg := workflowValidationConfig()
	cfg.Decisions[0].Workflow.WebSearch.MaxResults = 0
	if err := validateWorkflowContracts(cfg); err == nil || !strings.Contains(err.Error(), "max_results") {
		t.Fatalf("error = %v, want max_results failure", err)
	}
}

func TestValidateWorkflowContractsRejectsInvalidCredentialHeader(t *testing.T) {
	cfg := workflowValidationConfig()
	cfg.Decisions[0].Workflow.WebSearch.APIKeyEnv = "SEARCH_API_KEY"
	cfg.Decisions[0].Workflow.WebSearch.APIKeyHeader = "X-Search\r\nInjected"
	if err := validateWorkflowContracts(cfg); err == nil || !strings.Contains(err.Error(), "valid HTTP header") {
		t.Fatalf("error = %v, want header validation failure", err)
	}
}

func TestValidateWorkflowContractsAcceptsBoundedReadOnlyMCPWorkflow(t *testing.T) {
	cfg := workflowValidationConfig()
	cfg.Decisions[0].Workflow = testMCPWorkflowConfig()
	if err := validateWorkflowContracts(cfg); err != nil {
		t.Fatalf("validate MCP workflow: %v", err)
	}
}

func TestValidateWorkflowContractsRejectsMCPWithoutReadOnlyGuard(t *testing.T) {
	cfg := workflowValidationConfig()
	cfg.Decisions[0].Workflow = testMCPWorkflowConfig()
	cfg.Decisions[0].Workflow.MCP.RequireReadOnly = false
	if err := validateWorkflowContracts(cfg); err == nil || !strings.Contains(err.Error(), "require_read_only") {
		t.Fatalf("error = %v, want read-only guard failure", err)
	}
}

func TestValidateWorkflowContractsRejectsMismatchedWorkflowPayload(t *testing.T) {
	cfg := workflowValidationConfig()
	cfg.Decisions[0].Workflow.MCP = testMCPWorkflowConfig().MCP
	if err := validateWorkflowContracts(cfg); err == nil || !strings.Contains(err.Error(), "only web_search") {
		t.Fatalf("error = %v, want tagged-union failure", err)
	}
}

func testMCPWorkflowConfig() *WorkflowConfig {
	return &WorkflowConfig{
		Type: WorkflowMCPToolCall, AuthorizationGroup: "mcp-users",
		MCP: &MCPWorkflowConfig{
			ServerName: "catalog", Endpoint: "https://mcp.example.com", ToolName: "lookup",
			Arguments: map[string]interface{}{"query": "${user_content}"}, TimeoutSeconds: 5,
			MaxResponseBytes: 262144, MaxResultCharacters: 8000, RequireReadOnly: true,
		},
	}
}

func workflowValidationConfig() *RouterConfig {
	return &RouterConfig{IntelligentRouting: IntelligentRouting{Decisions: []Decision{{
		Name: "search",
		Workflow: &WorkflowConfig{
			Type: WorkflowWebSearchAnswer, AuthorizationGroup: "web-search-users",
			WebSearch: &WebSearchWorkflowConfig{
				Provider: WebSearchProviderSearXNG, Endpoint: "https://search.example.com/search",
				TimeoutSeconds: 5, MaxResults: 5, MaxQueryCharacters: 500,
				MaxResponseBytes: 262144, MaxEvidenceCharacters: 8000,
			},
		},
	}}}}
}
