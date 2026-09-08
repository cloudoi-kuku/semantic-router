package looper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/openai/openai-go"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
)

func TestFallbackLooperAdvancesAfterRetryableProviderFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-envoy-attempt-count", "2")
		if calls.Add(1) == 1 {
			http.Error(w, `{"error":"down"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("content-type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"ok","object":"chat.completion","model":"provider","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()

	algorithm := &config.AlgorithmConfig{Type: config.DecisionAlgorithmFallback, Fallback: &config.FallbackAlgorithmConfig{
		MaxAttempts: 2, RetryOn: []string{"server_error"},
	}}
	request := &Request{
		OriginalRequest: &openai.ChatCompletionNewParams{Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hello")}},
		ModelRefs:       []config.ModelRef{{Model: "primary"}, {Model: "secondary"}},
		Algorithm:       algorithm,
	}
	response, err := NewFallbackLooper(&config.LooperConfig{Endpoint: server.URL, TimeoutSeconds: 2}).Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("fallback execution failed: %v", err)
	}
	if response.Model != "secondary" || response.Iterations != 2 || calls.Load() != 2 {
		t.Fatalf("unexpected fallback response: model=%q iterations=%d calls=%d", response.Model, response.Iterations, calls.Load())
	}
	if response.ProviderAttempts != 4 || len(response.ModelsUsed) != 2 ||
		response.ModelsUsed[0] != "primary" || response.ModelsUsed[1] != "secondary" {
		t.Fatalf("unexpected attempt evidence: %+v", response)
	}
}

func TestFallbackLooperStopsOnNonRetryableFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
	}))
	defer server.Close()
	request := &Request{
		OriginalRequest: &openai.ChatCompletionNewParams{Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hello")}},
		ModelRefs:       []config.ModelRef{{Model: "primary"}, {Model: "secondary"}},
		Algorithm: &config.AlgorithmConfig{Type: config.DecisionAlgorithmFallback, Fallback: &config.FallbackAlgorithmConfig{
			MaxAttempts: 2, RetryOn: []string{"server_error"},
		}},
	}
	if _, err := NewFallbackLooper(&config.LooperConfig{Endpoint: server.URL, TimeoutSeconds: 2}).Execute(context.Background(), request); err == nil {
		t.Fatal("non-retryable provider failure was accepted")
	}
	if calls.Load() != 1 {
		t.Fatalf("non-retryable failure made %d calls, want 1", calls.Load())
	}
}
