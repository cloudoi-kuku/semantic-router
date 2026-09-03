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
