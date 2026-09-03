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
  capabilities: ["chat", "text", "tool-use"]
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
  MODEL "niffy-general" (reasoning = false)
  ALGORITHM static
}

ROUTE reasoning-route (description = "Select OpenAI for requests likely to need deliberate reasoning.") {
  PRIORITY 200
  WHEN keyword("reasoning_intent")
  MODEL "niffy-reasoning" (reasoning = true, effort = "medium")
  ALGORITHM static
}

ROUTE economical-default-route (description = "Select Mistral for all requests not matched by a more specific route.") {
  PRIORITY 100
  MODEL "niffy-cheap" (reasoning = false)
  ALGORITHM static
}
