package config

import (
	"fmt"
	"math"
	"strings"
)

func validateRequestBudgetContracts(cfg *RouterConfig) error {
	if cfg == nil {
		return nil
	}
	for _, decision := range cfg.AllRoutingDecisions() {
		budget := decision.RequestBudget
		if budget == nil {
			continue
		}
		if !currencyCodePattern.MatchString(strings.TrimSpace(budget.Currency)) {
			return fmt.Errorf("routing.decisions[%s].request_budget.currency must be a three-letter uppercase currency code", decision.Name)
		}
		if math.IsNaN(budget.MaxEstimatedCost) || math.IsInf(budget.MaxEstimatedCost, 0) || budget.MaxEstimatedCost <= 0 {
			return fmt.Errorf("routing.decisions[%s].request_budget.max_estimated_cost must be finite and greater than zero", decision.Name)
		}
		if budget.OutputTokenBound <= 0 {
			return fmt.Errorf("routing.decisions[%s].request_budget.output_token_bound must be greater than zero", decision.Name)
		}
		if budget.ReasoningTokenBound < 0 {
			return fmt.Errorf("routing.decisions[%s].request_budget.reasoning_token_bound must be non-negative", decision.Name)
		}
		if budget.RequireCurrentPricing && !budget.RequirePricing {
			return fmt.Errorf("routing.decisions[%s].request_budget.require_current_pricing requires require_pricing", decision.Name)
		}
	}
	return nil
}
