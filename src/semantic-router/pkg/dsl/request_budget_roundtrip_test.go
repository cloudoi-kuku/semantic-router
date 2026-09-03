package dsl

import (
	"strings"
	"testing"
)

func TestRequestBudgetCompileAndRoundTrip(t *testing.T) {
	source := `
ROUTE economical {
  PRIORITY 10
  REQUIRES ["chat"]
  BUDGET { currency: "USD", max_estimated_cost: 0.01, output_token_bound: 1024, reasoning_token_bound: 0, require_pricing: true, require_current_pricing: true }
  MODEL "cheap"
}`
	cfg, errs := Compile(source)
	if len(errs) > 0 {
		t.Fatalf("compile: %v", errs)
	}
	budget := cfg.Decisions[0].RequestBudget
	if budget == nil || budget.MaxEstimatedCost != 0.01 || budget.OutputTokenBound != 1024 || !budget.RequireCurrentPricing {
		t.Fatalf("budget = %#v", budget)
	}
	decompiled, err := Decompile(cfg)
	if err != nil {
		t.Fatalf("decompile: %v", err)
	}
	if !strings.Contains(decompiled, "BUDGET {") {
		t.Fatalf("decompiled DSL omitted budget:\n%s", decompiled)
	}
}
