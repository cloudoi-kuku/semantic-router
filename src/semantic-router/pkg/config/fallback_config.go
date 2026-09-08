package config

const ResilienceContractVersion = "vllm-sr/resilience-plan/v1alpha1"

var DefaultFallbackRetryOn = []string{
	"transport_error", "timeout", "rate_limited", "server_error", "invalid_response",
}
