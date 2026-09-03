# =============================================================================
# ROUTING PROFILE
# =============================================================================

ROUTING {
  strategy: priority
}

# =============================================================================
# SIGNALS
# =============================================================================

SIGNAL keyword tool_intent {
  operator: "OR"
  keywords: ["search the web", "web search", "browse the web", "look online", "find online", "latest news", "current news", "current information", "up-to-date", "real-time", "realtime", "use a tool", "call a tool", "use mcp", "mcp server"]
}

SIGNAL keyword reasoning_intent {
  operator: "OR"
  keywords: ["analyze", "reason through", "step by step", "prove", "derive", "calculate", "debug", "root cause", "design an architecture", "trade-off", "tradeoff", "compare approaches", "optimize", "multi-step", "implementation plan"]
}

# =============================================================================
# MODELS
# =============================================================================

MODEL niffy-cheap {
  description: "Economical default for direct questions, summaries, and routine chat."
  capabilities: ["chat", "text", "summarization"]
  tags: ["provider:mistral", "tier:cheap"]
  modality: "text"
}

MODEL niffy-general {
  description: "General model selected when a request indicates live-search or tool intent."
  capabilities: ["chat", "text", "tool_calling"]
  tags: ["provider:xai", "tier:general"]
  modality: "text"
}

MODEL niffy-reasoning {
  description: "Higher-cost model reserved for complex analysis and reasoning."
  capabilities: ["chat", "text", "reasoning", "code"]
  tags: ["provider:openai", "tier:reasoning"]
  modality: "text"
}

# =============================================================================
# ROUTES
# =============================================================================

ROUTE tool-intent-route (description = "Select Grok for requests that indicate search, freshness, or tool use.") {
  PRIORITY 300
  WHEN keyword("tool_intent")
  REQUIRES ["chat", "tool_calling"]
  BUDGET { currency: "USD", max_estimated_cost: 0.02, output_token_bound: 2048, reasoning_token_bound: 0, require_pricing: true, require_current_pricing: true }
  WORKFLOW { type: "web_search_answer", authorization_group: "web-search-users", provider: "searxng", endpoint: "http://host.docker.internal:8888/search", api_key_env: "", api_key_header: "", timeout_seconds: 5, max_results: 5, max_query_characters: 500, max_response_bytes: 262144, max_evidence_characters: 8000 }
  MODEL "niffy-general" (reasoning = false)
  ALGORITHM static
}

ROUTE reasoning-route (description = "Select OpenAI for requests likely to need deliberate reasoning.") {
  PRIORITY 200
  WHEN keyword("reasoning_intent")
  REQUIRES ["chat", "reasoning"]
  BUDGET { currency: "USD", max_estimated_cost: 0.12, output_token_bound: 4096, reasoning_token_bound: 4096, require_pricing: true, require_current_pricing: true }
  MODEL "niffy-reasoning" (reasoning = true, effort = "medium")
  ALGORITHM static
}

ROUTE economical-default-route (description = "Select Mistral for all requests not matched by a more specific route.") {
  PRIORITY 100
  REQUIRES ["chat"]
  BUDGET { currency: "USD", max_estimated_cost: 0.005, output_token_bound: 1024, reasoning_token_bound: 0, require_pricing: true, require_current_pricing: true }
  MODEL "niffy-cheap" (reasoning = false)
  ALGORITHM static
}
