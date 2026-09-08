package dsl

import (
	"strings"
	"testing"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
)

func TestFallbackAlgorithmRoundTrip(t *testing.T) {
	input := `
ROUTE resilient {
  PRIORITY 10
  MODEL "cheap", "alternate"
  ALGORITHM fallback {
    minimum_candidates: 2
    max_attempts: 2
    retry_on: ["timeout", "server_error"]
  }
}`
	cfg, errs := Compile(input)
	if len(errs) > 0 {
		t.Fatalf("compile errors: %v", errs)
	}
	algorithm := cfg.Decisions[0].Algorithm
	if algorithm == nil || algorithm.Fallback == nil ||
		algorithm.Fallback.MaxAttempts != 2 || len(algorithm.Fallback.RetryOn) != 2 {
		t.Fatalf("fallback algorithm = %#v", algorithm)
	}
	source, err := Decompile(&config.RouterConfig{IntelligentRouting: config.IntelligentRouting{Decisions: cfg.Decisions}})
	if err != nil {
		t.Fatalf("decompile error: %v", err)
	}
	for _, expected := range []string{"ALGORITHM fallback", "max_attempts: 2", `retry_on: ["timeout", "server_error"]`} {
		if !strings.Contains(source, expected) {
			t.Fatalf("decompiled source missing %q:\n%s", expected, source)
		}
	}
}
