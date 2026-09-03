package config

import (
	"strings"
	"testing"
)

func TestValidateRequestBudgetContracts(t *testing.T) {
	tests := []struct {
		name    string
		budget  *RequestBudget
		wantErr string
	}{
		{name: "valid", budget: &RequestBudget{Currency: "USD", MaxEstimatedCost: 0.01, OutputTokenBound: 1024, RequirePricing: true, RequireCurrentPricing: true}},
		{name: "zero ceiling", budget: &RequestBudget{Currency: "USD", OutputTokenBound: 1024}, wantErr: "max_estimated_cost"},
		{name: "missing output bound", budget: &RequestBudget{Currency: "USD", MaxEstimatedCost: 0.01}, wantErr: "output_token_bound"},
		{name: "freshness without pricing", budget: &RequestBudget{Currency: "USD", MaxEstimatedCost: 0.01, OutputTokenBound: 1024, RequireCurrentPricing: true}, wantErr: "requires require_pricing"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &RouterConfig{IntelligentRouting: IntelligentRouting{Decisions: []Decision{{Name: "route", RequestBudget: test.budget}}}}
			err := validateRequestBudgetContracts(cfg)
			if test.wantErr == "" && err != nil {
				t.Fatalf("validate: %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
	}
}
