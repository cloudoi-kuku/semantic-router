package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	workflowSecretEnvPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	workflowHeaderPattern    = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")
)

func validateWorkflowContracts(cfg *RouterConfig) error {
	if cfg == nil {
		return nil
	}
	for _, decision := range cfg.AllRoutingDecisions() {
		if decision.Workflow == nil {
			continue
		}
		if err := validateDecisionWorkflow(decision.Name, decision.Workflow); err != nil {
			return err
		}
	}
	return nil
}

func validateDecisionWorkflow(decisionName string, workflow *WorkflowConfig) error {
	prefix := fmt.Sprintf("routing.decisions[%s].workflow", decisionName)
	if strings.TrimSpace(workflow.AuthorizationGroup) == "" {
		return fmt.Errorf("%s.authorization_group is required", prefix)
	}
	switch workflow.Type {
	case WorkflowWebSearchAnswer:
		if workflow.WebSearch == nil || workflow.MCP != nil {
			return fmt.Errorf("%s must define only web_search for type %q", prefix, workflow.Type)
		}
		return validateWebSearchWorkflow(prefix+".web_search", workflow.WebSearch)
	case WorkflowMCPToolCall:
		if workflow.MCP == nil || workflow.WebSearch != nil {
			return fmt.Errorf("%s must define only mcp for type %q", prefix, workflow.Type)
		}
		return validateMCPWorkflow(prefix+".mcp", workflow.MCP)
	default:
		return fmt.Errorf("%s.type must be %q or %q", prefix, WorkflowWebSearchAnswer, WorkflowMCPToolCall)
	}
}

func validateMCPWorkflow(prefix string, workflow *MCPWorkflowConfig) error {
	if err := validateMCPIdentityAndEndpoint(prefix, workflow); err != nil {
		return err
	}
	if !workflow.RequireReadOnly {
		return fmt.Errorf("%s.require_read_only must be true", prefix)
	}
	if err := validateWorkflowCredential(prefix, workflow.APIKeyEnv, workflow.APIKeyHeader); err != nil {
		return err
	}
	return validateMCPBounds(prefix, workflow)
}

func validateMCPIdentityAndEndpoint(prefix string, workflow *MCPWorkflowConfig) error {
	if strings.TrimSpace(workflow.ServerName) == "" {
		return fmt.Errorf("%s.server_name is required", prefix)
	}
	if strings.TrimSpace(workflow.ToolName) == "" {
		return fmt.Errorf("%s.tool_name is required", prefix)
	}
	endpoint, err := url.ParseRequestURI(strings.TrimSpace(workflow.Endpoint))
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" {
		return fmt.Errorf("%s.endpoint must be an absolute HTTP(S) URL", prefix)
	}
	return nil
}

func validateMCPBounds(prefix string, workflow *MCPWorkflowConfig) error {
	if workflow.TimeoutSeconds < 1 || workflow.TimeoutSeconds > 30 {
		return fmt.Errorf("%s.timeout_seconds must be between 1 and 30", prefix)
	}
	if workflow.MaxResponseBytes < 1024 || workflow.MaxResponseBytes > 4*1024*1024 {
		return fmt.Errorf("%s.max_response_bytes must be between 1024 and 4194304", prefix)
	}
	if workflow.MaxResultCharacters < 128 || workflow.MaxResultCharacters > 20000 {
		return fmt.Errorf("%s.max_result_characters must be between 128 and 20000", prefix)
	}
	return nil
}

func validateWebSearchWorkflow(prefix string, search *WebSearchWorkflowConfig) error {
	if search.Provider != WebSearchProviderSearXNG {
		return fmt.Errorf("%s.provider must be %q", prefix, WebSearchProviderSearXNG)
	}
	endpoint, err := url.ParseRequestURI(strings.TrimSpace(search.Endpoint))
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" {
		return fmt.Errorf("%s.endpoint must be an absolute HTTP(S) URL", prefix)
	}
	if err := validateWebSearchCredential(prefix, search); err != nil {
		return err
	}
	return validateWebSearchBounds(prefix, search)
}

func validateWebSearchCredential(prefix string, search *WebSearchWorkflowConfig) error {
	return validateWorkflowCredential(prefix, search.APIKeyEnv, search.APIKeyHeader)
}

func validateWorkflowCredential(prefix, apiKeyEnv, apiKeyHeader string) error {
	if apiKeyEnv != "" && !workflowSecretEnvPattern.MatchString(apiKeyEnv) {
		return fmt.Errorf("%s.api_key_env must name an uppercase environment variable", prefix)
	}
	if apiKeyEnv != "" && strings.TrimSpace(apiKeyHeader) == "" {
		return fmt.Errorf("%s.api_key_header is required when api_key_env is set", prefix)
	}
	if apiKeyHeader != "" && !workflowHeaderPattern.MatchString(apiKeyHeader) {
		return fmt.Errorf("%s.api_key_header must be a valid HTTP header name", prefix)
	}
	return nil
}

func validateWebSearchBounds(prefix string, search *WebSearchWorkflowConfig) error {
	if search.TimeoutSeconds < 1 || search.TimeoutSeconds > 30 {
		return fmt.Errorf("%s.timeout_seconds must be between 1 and 30", prefix)
	}
	if search.MaxResults < 1 || search.MaxResults > 10 {
		return fmt.Errorf("%s.max_results must be between 1 and 10", prefix)
	}
	if search.MaxQueryCharacters < 1 || search.MaxQueryCharacters > 2000 {
		return fmt.Errorf("%s.max_query_characters must be between 1 and 2000", prefix)
	}
	if search.MaxResponseBytes < 1024 || search.MaxResponseBytes > 4*1024*1024 {
		return fmt.Errorf("%s.max_response_bytes must be between 1024 and 4194304", prefix)
	}
	if search.MaxEvidenceCharacters < 128 || search.MaxEvidenceCharacters > 20000 {
		return fmt.Errorf("%s.max_evidence_characters must be between 128 and 20000", prefix)
	}
	return nil
}
