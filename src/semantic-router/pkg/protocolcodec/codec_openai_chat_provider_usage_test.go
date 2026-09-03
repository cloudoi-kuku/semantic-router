package protocolcodec

import (
	"strings"
	"testing"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/llmprotocol"
)

func TestOpenAIChatResponseAcceptsProviderUsageMetadata(t *testing.T) {
	raw := []byte(`{
		"id":"chatcmpl-provider","object":"chat.completion","created":7,"model":"provider-model",
		"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"OK"}}],
		"usage":{"prompt_tokens":202,"completion_tokens":1,"total_tokens":522,
			"completion_tokens_details":{"reasoning_tokens":319},
			"service_tier":"standard","cost_in_usd_ticks":42,"num_sources_used":1}
	}`)
	translated, err := NewBuiltinEngine().TranslateResponse(
		llmprotocol.OpenAIChatV1,
		llmprotocol.AnthropicMessagesV1,
		raw,
		nil,
	)
	if err != nil {
		t.Fatalf("TranslateResponse() error = %v", err)
	}
	usage := translated.Response.Usage
	if usage.Total.Value == nil || *usage.Total.Value != 522 ||
		usage.OutputTotal.Value == nil || *usage.OutputTotal.Value != 320 ||
		usage.OutputReasoning.Value == nil || *usage.OutputReasoning.Value != 319 ||
		usage.OutputOther.Value == nil || *usage.OutputOther.Value != 1 {
		t.Fatalf("translated usage = %+v", translated.Response.Usage)
	}
	assertDiagnosticFields(
		t,
		translated.Diagnostics,
		"usage.cost_in_usd_ticks",
		"usage.num_sources_used",
		"usage.service_tier",
	)
}

func TestOpenAIChatResponseProviderUsageMetadataRemainsClosed(t *testing.T) {
	raw := []byte(`{
		"id":"chatcmpl-provider","object":"chat.completion","created":7,"model":"provider-model",
		"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"OK"}}],
		"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3,"future_cost_field":42}
	}`)
	_, err := NewBuiltinEngine().TranslateResponse(
		llmprotocol.OpenAIChatV1,
		llmprotocol.OpenAIChatV1,
		raw,
		func(*llmprotocol.Response) error { return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "invalid_upstream_json") {
		t.Fatalf("unknown provider usage field error = %v", err)
	}
}
