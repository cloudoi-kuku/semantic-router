package dsl

func rawToRoute(r *rawRouteDecl) *RouteDecl {
	route := &RouteDecl{Name: unquoteIdent(r.Name), Pos: posFromLexer(r.Pos)}
	applyRouteOptions(route, r.Opts)
	for _, item := range r.Body {
		applyRawRouteItem(route, item)
	}
	return route
}

func applyRawRouteItem(route *RouteDecl, item *rawRouteItem) {
	if applyRawRoutePolicyItem(route, item) {
		return
	}
	switch {
	case item.Model != nil:
		for _, model := range item.Model.Models {
			route.Models = append(route.Models, rawToModelRef(model))
		}
	case item.Algorithm != nil:
		route.Algorithm = rawToAlgo(item.Algorithm)
	case item.Plugin != nil:
		route.Plugins = append(route.Plugins, rawToPluginRef(item.Plugin))
	case item.Description != nil:
		route.Description = unquote(*item.Description)
	case item.Action != nil:
		route.Action = &ActionDecl{Type: item.Action.Type, Destination: unquoteIdent(item.Action.Destination), Pos: posFromLexer(item.Action.Pos)}
	case item.CandidateFor != nil:
		route.CandidateIterations = append(route.CandidateIterations, rawToCandidateIteration(item.CandidateFor))
	case item.Emit != nil:
		route.Emits = append(route.Emits, rawToEmitDecl(item.Emit))
	}
}

func applyRawRoutePolicyItem(route *RouteDecl, item *rawRouteItem) bool {
	switch {
	case item.Priority != nil:
		route.Priority = *item.Priority
	case item.Tier != nil:
		route.Tier = *item.Tier
	case item.When != nil:
		route.When = toBoolExpr(item.When)
	case item.Requires != nil:
		for _, capability := range item.Requires.Values {
			route.RequiredCapabilities = append(route.RequiredCapabilities, unquoteIdent(capability))
		}
	case item.Budget != nil:
		route.RequestBudget = rawBudgetToDecl(item.Budget)
	case item.Workflow != nil:
		route.Workflow = rawWorkflowToDecl(item.Workflow)
	default:
		return false
	}
	return true
}

func rawWorkflowToDecl(raw *rawWorkflowDecl) *WorkflowDecl {
	fields := entriesToMap(raw.Fields)
	workflow := &WorkflowDecl{}
	workflow.Type, _ = getStringField(fields, "type")
	workflow.AuthorizationGroup, _ = getStringField(fields, "authorization_group")
	workflow.Provider, _ = getStringField(fields, "provider")
	workflow.Endpoint, _ = getStringField(fields, "endpoint")
	workflow.APIKeyEnv, _ = getStringField(fields, "api_key_env")
	workflow.APIKeyHeader, _ = getStringField(fields, "api_key_header")
	workflow.TimeoutSeconds, _ = getIntField(fields, "timeout_seconds")
	workflow.MaxResults, _ = getIntField(fields, "max_results")
	workflow.MaxQueryCharacters, _ = getIntField(fields, "max_query_characters")
	workflow.MaxResponseBytes, _ = getIntField(fields, "max_response_bytes")
	workflow.MaxEvidenceCharacters, _ = getIntField(fields, "max_evidence_characters")
	workflow.ServerName, _ = getStringField(fields, "server_name")
	workflow.ToolName, _ = getStringField(fields, "tool_name")
	if arguments, ok := fields["arguments"].(ObjectValue); ok {
		workflow.Arguments = fieldsToMap(arguments.Fields)
	}
	workflow.MaxResultCharacters, _ = getIntField(fields, "max_result_characters")
	workflow.RequireReadOnly, _ = getBoolField(fields, "require_read_only")
	return workflow
}

func rawBudgetToDecl(raw *rawBudgetDecl) *RequestBudgetDecl {
	fields := entriesToMap(raw.Fields)
	budget := &RequestBudgetDecl{}
	budget.Currency, _ = getStringField(fields, "currency")
	budget.MaxEstimatedCost, _ = getFloat64Field(fields, "max_estimated_cost")
	budget.OutputTokenBound, _ = getIntField(fields, "output_token_bound")
	budget.ReasoningTokenBound, _ = getIntField(fields, "reasoning_token_bound")
	budget.RequirePricing, _ = getBoolField(fields, "require_pricing")
	budget.RequireCurrentPricing, _ = getBoolField(fields, "require_current_pricing")
	return budget
}
