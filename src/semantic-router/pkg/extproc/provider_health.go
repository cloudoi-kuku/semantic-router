package extproc

import (
	"strings"
	"time"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
)

type providerHealthState struct {
	consecutiveFailures int
	openUntil           time.Time
}

func (r *OpenAIRouter) observeProviderHealth(ctx *RequestContext, statusCode int) {
	model, policy, ok := r.providerHealthPolicy(ctx)
	if !ok || policy.CircuitBreakerFailures <= 0 {
		return
	}
	r.providerHealthMu.Lock()
	defer r.providerHealthMu.Unlock()
	if r.providerHealth == nil {
		r.providerHealth = make(map[string]providerHealthState)
	}
	state := r.providerHealth[model]
	if statusCode >= 200 && statusCode < 300 {
		delete(r.providerHealth, model)
		return
	}
	if !providerHealthFailure(statusCode) {
		return
	}
	state.consecutiveFailures++
	if state.consecutiveFailures >= policy.CircuitBreakerFailures {
		duration, err := time.ParseDuration(policy.CircuitBreakerOpenTime)
		if err == nil && duration > 0 {
			state.openUntil = time.Now().Add(duration)
		}
	}
	r.providerHealth[model] = state
}

func (r *OpenAIRouter) providerHealthPolicy(
	ctx *RequestContext,
) (string, config.ProviderReliability, bool) {
	if r == nil || r.Config == nil || ctx == nil {
		return "", config.ProviderReliability{}, false
	}
	model := strings.TrimSpace(ctx.VSRSelectedModel)
	if model == "" {
		model = strings.TrimSpace(ctx.RequestModel)
	}
	policy, ok := r.Config.GetProviderReliability(model)
	return model, policy, ok
}

func providerHealthFailure(statusCode int) bool {
	return statusCode == 408 || statusCode == 429 || statusCode >= 500
}

func (r *OpenAIRouter) modelCircuitOpen(model string, now time.Time) bool {
	if r == nil {
		return false
	}
	r.providerHealthMu.Lock()
	defer r.providerHealthMu.Unlock()
	state, ok := r.providerHealth[strings.TrimSpace(model)]
	if !ok || state.openUntil.IsZero() {
		return false
	}
	if !now.Before(state.openUntil) {
		delete(r.providerHealth, strings.TrimSpace(model))
		return false
	}
	return true
}
