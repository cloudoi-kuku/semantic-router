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
