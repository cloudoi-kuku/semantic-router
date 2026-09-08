package looper

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/observability/logging"
)

var defaultFallbackFailureClasses = []string{
	"transport_error", "timeout", "rate_limited", "server_error", "invalid_response",
}

// FallbackLooper tries an explicitly ordered provider-model chain and returns
// the first valid response. It never fans out and never exceeds max_attempts.
type FallbackLooper struct {
	base *BaseLooper
}

func NewFallbackLooper(cfg *config.LooperConfig) *FallbackLooper {
	return &FallbackLooper{base: NewBaseLooper(cfg)}
}

func (l *FallbackLooper) Execute(ctx context.Context, req *Request) (*Response, error) {
	if req == nil || len(req.ModelRefs) == 0 {
		return nil, fmt.Errorf("no fallback models configured")
	}
	if req.Algorithm == nil || req.Algorithm.Fallback == nil {
		return nil, fmt.Errorf("fallback algorithm configuration is unavailable")
	}
	cfg := req.Algorithm.Fallback
	maxAttempts := min(cfg.MaxAttempts, len(req.ModelRefs))
	retryOn := fallbackFailureSet(cfg.RetryOn)
	l.base.client.SetDecisionName(req.DecisionName)
	var lastErr error
	modelsUsed := make([]string, 0, maxAttempts)
	providerAttempts := 0
	for index, ref := range req.ModelRefs[:maxAttempts] {
		model := ref.Model
		if ref.LoRAName != "" {
			model = ref.LoRAName
		}
		accessKey := ""
		if params, ok := req.ModelParams[ref.Model]; ok {
			accessKey = params.AccessKey
		}
		attempt := index + 1
		modelsUsed = append(modelsUsed, model)
		response, err := l.base.callModelWithContextGate(
			ctx, req, req.OriginalRequest, model, req.IsStreaming, attempt, nil, accessKey,
		)
		if err == nil {
			providerAttempts += max(response.ProviderAttempts, 1)
			err = validateFallbackModelResponse(response)
		}
		if err == nil {
			logging.ComponentEvent("looper", "fallback_succeeded", map[string]interface{}{
				"decision": req.DecisionName, "model_ref": model, "attempt": attempt,
			})
			return fallbackResponse(response, modelsUsed, attempt, providerAttempts), nil
		}
		if response == nil {
			providerAttempts += fallbackProviderAttempts(err)
		}
		lastErr = err
		failureClass := fallbackFailureClass(err)
		logging.ComponentWarnEvent("looper", "fallback_attempt_failed", map[string]interface{}{
			"decision": req.DecisionName, "model_ref": model, "attempt": attempt,
			"failure_class": failureClass,
		})
		if _, allowed := retryOn[failureClass]; !allowed {
			return nil, fmt.Errorf("fallback stopped after model %s (%s): %w", model, failureClass, err)
		}
	}
	return nil, fmt.Errorf("fallback exhausted after %d attempts: %w", maxAttempts, lastErr)
}

func fallbackProviderAttempts(err error) int {
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) && statusErr.ProviderAttempts > 0 {
		return statusErr.ProviderAttempts
	}
	return 1
}

func fallbackFailureSet(configured []string) map[string]struct{} {
	if len(configured) == 0 {
		configured = defaultFallbackFailureClasses
	}
	result := make(map[string]struct{}, len(configured))
	for _, value := range configured {
		result[strings.TrimSpace(value)] = struct{}{}
	}
	return result
}

func fallbackFailureClass(err error) string {
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		switch {
		case statusErr.StatusCode == http.StatusRequestTimeout || statusErr.StatusCode == http.StatusGatewayTimeout:
			return "timeout"
		case statusErr.StatusCode == http.StatusTooManyRequests:
			return "rate_limited"
		case statusErr.StatusCode >= http.StatusInternalServerError:
			return "server_error"
		default:
			return "invalid_response"
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return "timeout"
		}
		return "transport_error"
	}
	if strings.Contains(err.Error(), "request failed") {
		return "transport_error"
	}
	return "invalid_response"
}

func validateFallbackModelResponse(response *ModelResponse) error {
	if response == nil {
		return fmt.Errorf("empty model response")
	}
	if response.IsStreaming {
		if len(response.StreamingChunks) == 0 {
			return fmt.Errorf("streaming model response contained no events")
		}
		return nil
	}
	if response.Parsed == nil || len(response.Parsed.Choices) == 0 {
		return fmt.Errorf("model response contained no choices")
	}
	return nil
}

func fallbackResponse(
	response *ModelResponse,
	modelsUsed []string,
	attempts int,
	providerAttempts int,
) *Response {
	contentType := "application/json"
	if response.IsStreaming {
		contentType = "text/event-stream"
	}
	return &Response{
		Body: response.Raw, ContentType: contentType, Model: response.Model,
		ModelsUsed: append([]string(nil), modelsUsed...), Iterations: attempts,
		AlgorithmType: config.DecisionAlgorithmFallback, Usage: response.Usage,
		ProviderAttempts: providerAttempts,
	}
}
