package dsl

import (
	"strings"
	"testing"
)

func TestWebSearchWorkflowCompileAndRoundTrip(t *testing.T) {
	source := `
MODEL "synth" { backend_model: "model", url: "https://example.com/v1", api_key: "test" }
ROUTE "search" {
  WHEN keyword("fresh")
  WORKFLOW { type: "web_search_answer", authorization_group: "web-search-users", provider: "searxng", endpoint: "https://search.example.com/search", api_key_env: "WEB_SEARCH_API_KEY", api_key_header: "X-Search-Token", timeout_seconds: 5, max_results: 5, max_query_characters: 500, max_response_bytes: 262144, max_evidence_characters: 8000 }
  MODEL "synth"
}
SIGNAL keyword "fresh" { keywords: ["latest"] }
`
	cfg, errs := Compile(source)
	if len(errs) > 0 {
		t.Fatalf("compile: %v", errs)
	}
	workflow := cfg.Decisions[0].Workflow
	if workflow == nil || workflow.WebSearch == nil || workflow.WebSearch.Provider != "searxng" {
		t.Fatalf("compiled workflow = %+v", workflow)
	}
	decompiled, err := Decompile(cfg)
	if err != nil {
		t.Fatalf("decompile: %v", err)
	}
	if !strings.Contains(decompiled, "WORKFLOW {") || !strings.Contains(decompiled, `authorization_group: "web-search-users"`) {
		t.Fatalf("decompiled workflow missing: %s", decompiled)
	}
}

func TestMCPWorkflowCompileAndRoundTrip(t *testing.T) {
	source := `
MODEL "synth" { backend_model: "model", url: "https://example.com/v1", api_key: "test" }
ROUTE "mcp" {
  WHEN keyword("tool")
  WORKFLOW { type: "mcp_tool_call", authorization_group: "mcp-users", server_name: "catalog", endpoint: "https://mcp.example.com", tool_name: "lookup", arguments: { query: "{{user_content}}" }, api_key_env: "MCP_API_KEY", api_key_header: "Authorization", timeout_seconds: 5, max_response_bytes: 262144, max_result_characters: 8000, require_read_only: true }
  MODEL "synth"
}
SIGNAL keyword "tool" { keywords: ["lookup"] }
`
	cfg, errs := Compile(source)
	if len(errs) > 0 {
		t.Fatalf("compile: %v", errs)
	}
	workflow := cfg.Decisions[0].Workflow
	if workflow == nil || workflow.MCP == nil || workflow.MCP.Arguments["query"] != "{{user_content}}" {
		t.Fatalf("compiled workflow = %+v", workflow)
	}
	decompiled, err := Decompile(cfg)
	if err != nil {
		t.Fatalf("decompile: %v", err)
	}
	if !strings.Contains(decompiled, `type: "mcp_tool_call"`) || !strings.Contains(decompiled, `arguments: { query: "{{user_content}}" }`) {
		t.Fatalf("decompiled workflow missing MCP fields: %s", decompiled)
	}
}
