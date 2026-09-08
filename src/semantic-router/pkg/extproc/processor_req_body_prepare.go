package extproc

import (
	"context"
	"errors"
	"fmt"

	ext_proc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/decision"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/inflight"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/llmprotocol"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/observability/logging"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/observability/metrics"
	"github.com/vllm-project/semantic-router/src/semantic-router/pkg/routerreplay"
)

func (r *OpenAIRouter) extractRequestSignalSnapshot(
	ctx *RequestContext,
) (*requestSignalSnapshot, error) {
	if ctx == nil || ctx.SemanticRequest == nil {
		return nil, status.Error(codes.InvalidArgument, "neutral inference request is unavailable")
	}
	snapshot := extractSemanticRequestSignals(ctx.SemanticRequest)
	if snapshot.Stream {
		logging.ComponentDebugEvent("extproc", "stream_parameter_detected", map[string]interface{}{
			"request_id": ctx.RequestID,
		})
		ctx.ExpectStreamingResponse = true
	}
	return snapshot, nil
}

func (r *OpenAIRouter) runRequestPreRoutingStages(
	originalModel string,
	snapshot *requestSignalSnapshot,
	ctx *RequestContext,
) (requestDecisionState, *ext_proc.ProcessingResponse) {
	if !ctx.Routing.IsResolved() {
		r.resolveEntrypointForRequest(originalModel, ctx)
	}
	populatePinnedSessionFromHeaders(ctx)
	history := signalConversationHistoryFromSnapshot(snapshot)
	applyRequestContextEstimate(snapshot, ctx)
	if !r.requestModelActsAsAuto(originalModel) {
		if policyErr := r.validateTenantPolicyModel(originalModel, nil, ctx); policyErr != nil {
			logging.Warnf("[Request Body] Explicit model failed tenant policy: %v", policyErr)
			return requestDecisionState{}, r.createErrorResponse(422, policyErr.Error())
		}
	}
	state, response := r.resolveRequestDecision(
		originalModel,
		history,
		ctx,
	)
	if response != nil {
		return requestDecisionState{}, response
	}
	decisionName := state.decisionName
	selectedModel := state.selectedModel
	metrics.RecordModelRequest(selectedModel)
	ctx.InflightToken = inflight.Begin(selectedModel)
	if resp := r.handleFastResponse(ctx, decisionName); resp != nil {
		endRequestInflight(ctx, selectedModel)
		r.startRouterReplay(ctx, originalModel, selectedModel, decisionName)
		r.updateRouterReplayStatus(ctx, 200, false)
		r.attachRouterReplayResponse(
			ctx,
			resp.GetImmediateResponse().GetBody(),
			true,
		)
		addRouterReplayHeaderToImmediateResponse(resp, ctx.RouterReplayID)
		return requestDecisionState{}, resp
	}
	if resp := r.applyRateLimit(ctx, selectedModel); resp != nil {
		endRequestInflight(ctx, selectedModel)
		return requestDecisionState{}, resp
	}
	if resp := r.applyCacheChecks(ctx, selectedModel, decisionName); resp != nil {
		endRequestInflight(ctx, selectedModel)
		return requestDecisionState{}, resp
	}
	if workflowErr := r.executeDecisionWorkflow(ctx, decisionName); workflowErr != nil {
		endRequestInflight(ctx, selectedModel)
		statusCode := workflowErrorStatus(workflowErr)
		return requestDecisionState{}, r.createErrorResponse(statusCode, workflowErr.Error())
	}
	if ragErr := r.executeRAGPlugin(ctx, decisionName); ragErr != nil {
		endRequestInflight(ctx, selectedModel)
		return requestDecisionState{}, r.createErrorResponse(503, fmt.Sprintf("RAG retrieval failed: %v", ragErr))
	}

	return state, nil
}

func (r *OpenAIRouter) resolveRequestDecision(
	originalModel string,
	history signalConversationHistory,
	ctx *RequestContext,
) (requestDecisionState, *ext_proc.ProcessingResponse) {
	decisionName, _, reasoningDecision, selectedModel, err := r.performDecisionEvaluation(
		originalModel,
		history,
		ctx,
	)
	if err == nil {
		return requestDecisionState{
			decisionName:      decisionName,
			reasoningDecision: reasoningDecision,
			selectedModel:     selectedModel,
		}, nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return requestDecisionState{}, r.createErrorResponse(499, "request canceled")
	}
	if errors.Is(err, errNoContextEligibleDecisionModel) {
		logging.Warnf("[Request Body] Decision candidates failed request eligibility: %v", err)
		return requestDecisionState{}, r.createErrorResponse(422, err.Error())
	}
	logging.Errorf("[Request Body] Decision evaluation failed: %v", err)
	if errors.Is(err, decision.ErrDecisionUnresolved) {
		return requestDecisionState{}, r.respondDecisionUnresolved(ctx, originalModel, err)
	}
	return requestDecisionState{}, r.createErrorResponse(403, err.Error())
}

func endRequestInflight(ctx *RequestContext, selectedModel string) {
	inflight.End(selectedModel, ctx.InflightToken)
	ctx.InflightToken = 0
}

// respondDecisionUnresolved builds the fail_request 503 and finalizes the
// replay record as failed, matching the looper-failure path.
func (r *OpenAIRouter) respondDecisionUnresolved(
	ctx *RequestContext,
	originalModel string,
	decisionErr error,
) *ext_proc.ProcessingResponse {
	resp := r.createErrorResponse(503, decisionErr.Error())
	if ctx.RouterReplayPluginConfig == nil {
		ctx.RouterReplayPluginConfig = r.Config.EffectiveRouterReplayConfig(nil)
	}
	r.startRouterReplay(ctx, originalModel, "", "")
	r.updateRouterReplayStatus(ctx, 503, false)
	if immediate := resp.GetImmediateResponse(); immediate != nil {
		r.attachRouterReplayResponse(ctx, immediate.Body, false)
	}
	// Failed, not aborted: the router itself rejected the request with a
	// terminal 503; aborted is reserved for streams that end early.
	r.finalizeRouterReplay(ctx, routerreplay.LifecycleFailed, "decision_unresolved")
	addRouterReplayHeaderToImmediateResponse(resp, ctx.RouterReplayID)
	return resp
}

func applyRequestContextEstimate(snapshot *requestSignalSnapshot, ctx *RequestContext) {
	if snapshot == nil || ctx == nil {
		return
	}
	ctx.VSRContextTokenCount = snapshot.ContextTokenFloor
	ctx.VSRContextTextBytes = snapshot.ContextTextBytes
	ctx.VSRContextEquivalentBytes = snapshot.ContextEquivalentBytes
	ctx.VSRContextHasNonText = snapshot.ContextHasNonText
	if ctx.SemanticRequest != nil {
		if bound := ctx.SemanticRequest.Sampling.MaxOutputTokens; bound != nil && *bound > 0 {
			ctx.VSROutputTokenBound = int(*bound)
		}
		if bound := ctx.SemanticRequest.ReasoningBudgetTokens; bound != nil && *bound > 0 {
			ctx.VSRReasoningTokenBound = int(*bound)
		}
	}
}

func (r *OpenAIRouter) applyCacheChecks(
	ctx *RequestContext,
	selectedModel string,
	decisionName string,
) *ext_proc.ProcessingResponse {
	if response, shouldReturn := r.handleCaching(ctx, decisionName, selectedModel); shouldReturn {
		logging.ComponentDebugEvent("extproc", "cache_short_circuit", map[string]interface{}{
			"request_id": ctx.RequestID,
			"decision":   decisionName,
		})
		return response
	}
	return nil
}

func (r *OpenAIRouter) prepareRequestForModelRouting(
	request *llmprotocol.Request,
	userContent string,
	ctx *RequestContext,
) (*llmprotocol.Request, *ext_proc.ProcessingResponse, error) {
	if request == nil {
		return nil, nil, status.Error(codes.InvalidArgument, "neutral inference request is unavailable")
	}
	populateSessionTransitionFields(ctx)
	memErr := r.handleMemoryRetrieval(ctx, userContent, request)
	if memErr != nil {
		logging.ComponentWarnEvent("extproc", "memory_retrieval_failed", map[string]interface{}{
			"request_id": ctx.RequestID,
			"error":      memErr.Error(),
			"fallback":   "continue_without_memory",
		})
	}
	if compressionErr := r.applySemanticContextCompression(ctx, request); compressionErr != nil {
		return nil, r.createErrorResponse(500, "Context compression failed under fail_closed policy"), nil
	}
	return request, nil, nil
}
