package extproc

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/llmprotocol"
)

func TestExecuteDecisionWorkflowAuthorizesSearchAndInjectsBoundedEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if got := req.URL.Query().Get("q"); got != "current release status" {
			t.Fatalf("query = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]string{
			{"title": "Release", "url": "https://example.com/release", "content": strings.Repeat("evidence ", 100)},
			{"title": "Ignored", "url": "javascript:alert(1)", "content": "unsafe URL"},
		}})
	}))
	defer server.Close()

	workflow := testWebSearchWorkflow(server.URL)
	router := &OpenAIRouter{Config: &config.RouterConfig{Authz: config.AuthzConfig{
		Identity: config.IdentityConfig{UserGroupsHeader: "x-authz-user-groups"},
	}}}
	ctx := &RequestContext{
		Headers:     map[string]string{"x-authz-user-groups": "users,web-search-users"},
		UserContent: "current release status",
		SemanticRequest: &llmprotocol.Request{Messages: []llmprotocol.Message{{
			Role: llmprotocol.RoleUser, Content: []llmprotocol.Content{{Kind: llmprotocol.ContentText, Text: "current release status"}},
		}}},
		VSRSelectedDecision: &config.Decision{Name: "search", Workflow: workflow},
	}

	if err := router.executeDecisionWorkflow(ctx, "search"); err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if ctx.VSRWorkflowStatus != "completed" || ctx.VSRWorkflowEvidenceCount != 1 {
		t.Fatalf("workflow evidence state = %q/%d", ctx.VSRWorkflowStatus, ctx.VSRWorkflowEvidenceCount)
	}
	if len(ctx.SemanticRequest.Messages) != 3 {
		t.Fatalf("message count = %d, want 3", len(ctx.SemanticRequest.Messages))
	}
	injected := ctx.ToolResultsContext
	if len([]rune(injected)) > workflow.WebSearch.MaxEvidenceCharacters {
		t.Fatalf("injected evidence exceeds configured bound: %d", len([]rune(injected)))
	}
	for _, want := range []string{"web_search_evidence", `trust="untrusted"`, "https://example.com/release"} {
		if !strings.Contains(injected, want) {
			t.Fatalf("injected evidence missing %q: %s", want, injected)
		}
	}
}

func TestExecuteDecisionWorkflowFailsClosedWithoutAuthorizedGroup(t *testing.T) {
	workflow := testWebSearchWorkflow("https://search.example.com")
	router := &OpenAIRouter{Config: &config.RouterConfig{}}
	ctx := &RequestContext{
		Headers: map[string]string{"x-authz-user-groups": "users"}, UserContent: "news",
		SemanticRequest:     &llmprotocol.Request{},
		VSRSelectedDecision: &config.Decision{Name: "search", Workflow: workflow},
	}
	err := router.executeDecisionWorkflow(ctx, "search")
	if err == nil || !strings.Contains(err.Error(), "authorization denied") {
		t.Fatalf("error = %v, want authorization denial", err)
	}
	if status := workflowErrorStatus(err); status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", status)
	}
}

func TestExecuteDecisionWorkflowFailsClosedWithoutUsableEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]string{{
			"title": "Rejected", "url": "javascript:alert(1)", "content": "not usable",
		}}})
	}))
	defer server.Close()

	router := &OpenAIRouter{Config: &config.RouterConfig{Authz: config.AuthzConfig{
		Identity: config.IdentityConfig{UserGroupsHeader: "x-authz-user-groups"},
	}}}
	ctx := &RequestContext{
		Headers:             map[string]string{"x-authz-user-groups": "web-search-users"},
		UserContent:         "current status",
		SemanticRequest:     &llmprotocol.Request{},
		VSRSelectedDecision: &config.Decision{Name: "search", Workflow: testWebSearchWorkflow(server.URL)},
	}
	if err := router.executeDecisionWorkflow(ctx, "search"); err == nil || !strings.Contains(err.Error(), "no usable evidence") {
		t.Fatalf("error = %v, want no usable evidence failure", err)
	}
	if ctx.VSRWorkflowStatus != "failed" {
		t.Fatalf("workflow status = %q, want failed", ctx.VSRWorkflowStatus)
	}
}

func TestExecuteDecisionWorkflowDiscoversAndCallsReadOnlyMCPTool(t *testing.T) {
	var paths []string
	server := newReadOnlyMCPTestServer(t, &paths)
	defer server.Close()

	router := &OpenAIRouter{Config: &config.RouterConfig{Authz: config.AuthzConfig{Identity: config.IdentityConfig{UserGroupsHeader: "x-authz-user-groups"}}}}
	ctx := &RequestContext{
		Headers: map[string]string{"x-authz-user-groups": "mcp-users"}, UserContent: "find current account",
		SemanticRequest: &llmprotocol.Request{}, VSRSelectedDecision: &config.Decision{Name: "mcp", Workflow: testMCPWorkflow(server.URL)},
	}
	if err := router.executeDecisionWorkflow(ctx, "mcp"); err != nil {
		t.Fatalf("execute MCP workflow: %v", err)
	}
	if !strings.Contains(ctx.ToolResultsContext, `server="catalog" tool="lookup" trust="untrusted"`) ||
		!strings.Contains(ctx.ToolResultsContext, "account status is active") {
		t.Fatalf("injected result = %s", ctx.ToolResultsContext)
	}
	if got := strings.Join(paths, ","); !strings.Contains(got, "/tools/list") || !strings.HasSuffix(got, "/tools/call") {
		t.Fatalf("MCP request paths = %s", got)
	}
}

func newReadOnlyMCPTestServer(t *testing.T, paths *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		*paths = append(*paths, req.URL.Path)
		switch req.URL.Path {
		case "/initialize":
			_, _ = w.Write([]byte(`{}`))
		case "/tools/list":
			_, _ = w.Write([]byte(`{"tools":[{"name":"lookup","inputSchema":{"type":"object","required":["query"]},"annotations":{"readOnlyHint":true}}]}`))
		case "/resources/list":
			_, _ = w.Write([]byte(`{"resources":[]}`))
		case "/prompts/list":
			_, _ = w.Write([]byte(`{"prompts":[]}`))
		case "/tools/call":
			var request struct {
				Params struct {
					Arguments map[string]interface{} `json:"arguments"`
				} `json:"params"`
			}
			_ = json.NewDecoder(req.Body).Decode(&request)
			if request.Params.Arguments["query"] != "find current account" {
				t.Fatalf("arguments = %#v", request.Params.Arguments)
			}
			_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"account status is active"}]}`))
		default:
			http.NotFound(w, req)
		}
	}))
}

func TestExecuteDecisionWorkflowRejectsMCPToolWithoutReadOnlyAnnotation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/initialize":
			_, _ = w.Write([]byte(`{}`))
		case "/tools/list":
			_, _ = w.Write([]byte(`{"tools":[{"name":"lookup","inputSchema":{"type":"object"},"annotations":{}}]}`))
		case "/resources/list":
			_, _ = w.Write([]byte(`{"resources":[]}`))
		case "/prompts/list":
			_, _ = w.Write([]byte(`{"prompts":[]}`))
		default:
			t.Fatalf("unexpected execution request %s", req.URL.Path)
		}
	}))
	defer server.Close()
	router := &OpenAIRouter{Config: &config.RouterConfig{Authz: config.AuthzConfig{Identity: config.IdentityConfig{UserGroupsHeader: "x-authz-user-groups"}}}}
	ctx := &RequestContext{Headers: map[string]string{"x-authz-user-groups": "mcp-users"}, SemanticRequest: &llmprotocol.Request{}, VSRSelectedDecision: &config.Decision{Workflow: testMCPWorkflow(server.URL)}}
	if err := router.executeDecisionWorkflow(ctx, "mcp"); err == nil || !strings.Contains(err.Error(), "readOnlyHint=true") {
		t.Fatalf("error = %v, want read-only discovery rejection", err)
	}
}

func TestExecuteDecisionWorkflowDoesNotDiscoverMCPBeforeAuthorization(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		http.Error(w, "must not be called", http.StatusInternalServerError)
	}))
	defer server.Close()
	router := &OpenAIRouter{Config: &config.RouterConfig{Authz: config.AuthzConfig{Identity: config.IdentityConfig{UserGroupsHeader: "x-authz-user-groups"}}}}
	ctx := &RequestContext{Headers: map[string]string{"x-authz-user-groups": "other"}, SemanticRequest: &llmprotocol.Request{}, VSRSelectedDecision: &config.Decision{Workflow: testMCPWorkflow(server.URL)}}
	if err := router.executeDecisionWorkflow(ctx, "mcp"); !errors.Is(err, errWorkflowUnauthorized) {
		t.Fatalf("error = %v, want authorization denial", err)
	}
	if requests != 0 {
		t.Fatalf("MCP requests before authorization = %d", requests)
	}
}

func testMCPWorkflow(endpoint string) *config.WorkflowConfig {
	return &config.WorkflowConfig{Type: config.WorkflowMCPToolCall, AuthorizationGroup: "mcp-users", MCP: &config.MCPWorkflowConfig{
		ServerName: "catalog", Endpoint: endpoint, ToolName: "lookup", Arguments: map[string]interface{}{"query": "${user_content}"},
		TimeoutSeconds: 2, MaxResponseBytes: 8192, MaxResultCharacters: 300, RequireReadOnly: true,
	}}
}

func testWebSearchWorkflow(endpoint string) *config.WorkflowConfig {
	return &config.WorkflowConfig{
		Type: config.WorkflowWebSearchAnswer, AuthorizationGroup: "web-search-users",
		WebSearch: &config.WebSearchWorkflowConfig{
			Provider: config.WebSearchProviderSearXNG, Endpoint: endpoint,
			TimeoutSeconds: 2, MaxResults: 3, MaxQueryCharacters: 100,
			MaxResponseBytes: 8192, MaxEvidenceCharacters: 300,
		},
	}
}
