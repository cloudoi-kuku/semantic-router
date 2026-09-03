/*
Copyright 2025 vLLM Semantic Router.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package extproc

import (
	"fmt"
	"time"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/config"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/observability/logging"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/utils/entropy"
)

func (r *OpenAIRouter) selectDecisionDefaultRuntimeModel(
	decisionConfig *config.Decision,
	decisionName string,
	ctx *RequestContext,
) (string, entropy.ReasoningDecision, error) {
	if err := validateMinimumEligibleDecisionModels(
		decisionConfig,
		nil,
		ctx.VSRContextTokenCount,
	); err != nil {
		return "", entropy.ReasoningDecision{}, err
	}
	selectedModel := r.Config.DefaultModel
	if budget := decisionConfig.RequestBudget; budget != nil {
		budgetResult := r.requestBudgetEligibleModelRefs(
			[]config.ModelRef{{Model: selectedModel}}, budget,
			ctx.VSRContextTokenCount, ctx.VSROutputTokenBound, ctx.VSRReasoningTokenBound, time.Now().UTC(),
		)
		if len(budgetResult.eligible) == 0 {
			return "", entropy.ReasoningDecision{}, fmt.Errorf(
				"%w: configured default model for decision %q failed request-budget eligibility",
				errNoContextEligibleDecisionModel, decisionName,
			)
		}
	}
	if r.modelNameExceedsContextWindow(selectedModel, ctx.VSRContextTokenCount) {
		return "", entropy.ReasoningDecision{}, fmt.Errorf(
			"%w: decision %q requires %d request tokens but the configured default model has a smaller context window",
			errNoContextEligibleDecisionModel,
			decisionName,
			ctx.VSRContextTokenCount,
		)
	}
	if missing := r.modelNameMissingCapabilities(selectedModel, decisionConfig.RequiredCapabilities); len(missing) > 0 {
		return "", entropy.ReasoningDecision{}, fmt.Errorf(
			"%w: configured default model for decision %q is missing required capabilities %v",
			errNoContextEligibleDecisionModel,
			decisionName,
			missing,
		)
	}
	ctx.VSRSelectedModel = selectedModel
	ctx.VSRSelectionMethod = "default"
	logging.ComponentDebugEvent("extproc", "decision_model_defaulted", map[string]interface{}{
		"request_id":     ctx.RequestID,
		"decision":       decisionName,
		"selected_model": selectedModel,
	})
	return selectedModel, entropy.ReasoningDecision{}, nil
}
