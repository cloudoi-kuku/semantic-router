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

SIGNAL embedding live_information_task {
  threshold: 0.56
  candidates: ["retrieve recent information from the internet before answering", "verify a current claim using live external sources", "find today's status, price, schedule, or breaking news", "look up information that may have changed recently"]
  aggregation_method: "max"
}

SIGNAL complexity reasoning_demand {
  threshold: 0.10
  description: "Calibrated local prototype margin for deliberate reasoning demand."
  hard: { candidates: ["compare multiple approaches and justify the tradeoffs", "diagnose a complex failure from several interacting causes", "derive a result step by step from first principles", "synthesize constraints into an implementation architecture"] }
  easy: { candidates: ["give a short direct factual answer", "rewrite or translate one sentence", "summarize a short paragraph briefly", "provide a simple definition"] }
}

# =============================================================================
# MODELS
# =============================================================================

MODEL niffy-cheap {
  description: "Economical default for direct questions, summaries, and routine chat."
  capabilities: ["chat", "text", "summarization", "code"]
  tags: ["provider:mistral", "tier:cheap"]
  modality: "text"
}

MODEL niffy-general {
  description: "General model selected when a request indicates live-search or tool intent."
  capabilities: ["chat", "text", "code", "tool_calling"]
  tags: ["provider:xai", "tier:general"]
  modality: "text"
}

MODEL niffy-reasoning {
  description: "Higher-cost model reserved for complex analysis and reasoning."
  capabilities: ["chat", "text", "reasoning", "code", "tool_calling"]
  tags: ["provider:openai", "tier:reasoning"]
  modality: "text"
}

# =============================================================================
# ROUTES
# =============================================================================

ROUTE tool-intent-route (description = "Select Grok for requests that indicate search, freshness, or tool use.") {
  PRIORITY 300
  WHEN keyword("tool_intent") OR embedding("live_information_task")
  REQUIRES ["chat", "tool_calling"]
  BUDGET { currency: "USD", max_estimated_cost: 0.02, output_token_bound: 2048, reasoning_token_bound: 0, require_pricing: true, require_current_pricing: true }
  WORKFLOW { type: "web_search_answer", authorization_group: "web-search-users", provider: "searxng", endpoint: "http://host.docker.internal:8888/search", api_key_env: "", api_key_header: "", timeout_seconds: 5, max_results: 5, max_query_characters: 500, max_response_bytes: 262144, max_evidence_characters: 8000 }
  MODEL "niffy-general" (reasoning = false)
  ALGORITHM static
  PLUGIN router_replay {
    enabled: true
    max_records: 1000
    capture_request_body: false
    capture_response_body: false
  }
}

ROUTE reasoning-route (description = "Select OpenAI for requests likely to need deliberate reasoning.") {
  PRIORITY 200
  WHEN keyword("reasoning_intent") OR complexity("reasoning_demand:hard")
  REQUIRES ["chat", "reasoning"]
  BUDGET { currency: "USD", max_estimated_cost: 0.25, output_token_bound: 4096, reasoning_token_bound: 4096, require_pricing: true, require_current_pricing: true }
  MODEL "niffy-reasoning" (reasoning = true, effort = "medium")
  ALGORITHM static
  PLUGIN router_replay {
    enabled: true
    max_records: 1000
    capture_request_body: false
    capture_response_body: false
  }
}

ROUTE economical-default-route (description = "Select Mistral for all requests not matched by a more specific route.") {
  PRIORITY 100
  REQUIRES ["chat"]
  BUDGET { currency: "USD", max_estimated_cost: 0.01, output_token_bound: 1024, reasoning_token_bound: 0, require_pricing: true, require_current_pricing: true }
  MODEL "niffy-cheap" (reasoning = false)
  MODEL "niffy-general" (reasoning = false)
  ALGORITHM fallback { minimum_candidates: 2, max_attempts: 2, retry_on: ["transport_error", "timeout", "rate_limited", "server_error", "invalid_response"] }
  PLUGIN router_replay {
    enabled: true
    max_records: 1000
    capture_request_body: false
    capture_response_body: false
  }
}

# =============================================================================
# ENTRYPOINTS
# =============================================================================

ENTRYPOINT {
  model_names: ["niffy/code"]
  recipe: "code"
}

# =============================================================================
# RECIPE code
# =============================================================================

RECIPE code (description = "Provider-neutral coding policy with evidence-gated premium escalation. Tool execution remains the responsibility of an authorized caller.") {
  ROUTING {
    strategy: priority
  }

  SIGNAL keyword code_tool_task {
    operator: "OR"
    keywords: ["edit the file", "modify the code", "apply a patch", "inspect the repository", "search the codebase", "run the tests", "run the build", "execute the command", "use the terminal", "use mcp", "call a tool"]
  }

  SIGNAL keyword code_general_task {
    operator: "OR"
    keywords: ["implement", "write code", "generate code", "fix this bug", "refactor", "add unit tests", "write a unit test", "write a test", "create a function", "add an endpoint", "update this class"]
  }

  SIGNAL keyword code_reasoning_task {
    operator: "OR"
    keywords: ["architecture", "security review", "threat model", "race condition", "deadlock", "distributed system", "root cause", "data migration", "breaking change", "compare approaches", "prove correctness", "multiple services"]
  }

  SIGNAL context code_large_context {
    description: "Large repository or multi-file context that needs stronger synthesis."
    min_tokens: "12K"
    max_tokens: "262K"
  }

  SIGNAL complexity code_reasoning_demand {
    threshold: 0.20
    description: "Calibrated local prototype margin for complex coding work."
    hard: { candidates: ["diagnose an intermittent concurrency failure across several components", "design a secure migration while preserving compatibility and rollback", "compare implementation architectures and justify their tradeoffs", "reason about correctness across multiple files and services"] }
    easy: { candidates: ["explain one short code fragment", "rename a variable in one function", "write a small deterministic helper", "summarize a compiler error briefly"] }
  }

  SIGNAL authz code_validator {
    role: "code_validator"
    description: "Trusted coding orchestrators allowed to submit validation outcomes."
    subjects: [{ kind: "Group", name: "code-validation-producers" }]
  }

  SIGNAL metadata code_validation_failed {
    description: "Objective patch, build, test, or review evidence reported a failure."
    key: "niffy.code.validation"
    predicate: { in: ["patch_failed", "build_failed", "test_failed", "review_failed"] }
  }

  ROUTE code-evidence-escalation-route (description = "Escalate only when a trusted orchestrator reports objective validation failure.") {
    PRIORITY 500
    WHEN authz("code_validator") AND metadata("code_validation_failed")
    REQUIRES ["chat", "code", "reasoning"]
    BUDGET { currency: "USD", max_estimated_cost: 0.25, output_token_bound: 4096, reasoning_token_bound: 4096, require_pricing: true, require_current_pricing: true }
    MODEL "niffy-reasoning" (reasoning = true, effort = "medium")
    ALGORITHM static
    PLUGIN router_replay {
      enabled: true
      max_records: 1000
      capture_request_body: false
      capture_response_body: false
    }
  }

  ROUTE code-reasoning-route (description = "Route intrinsically complex coding work to the reasoning tier.") {
    PRIORITY 400
    WHEN keyword("code_reasoning_task") OR complexity("code_reasoning_demand:hard") OR context("code_large_context")
    REQUIRES ["chat", "code", "reasoning"]
    BUDGET { currency: "USD", max_estimated_cost: 0.25, output_token_bound: 4096, reasoning_token_bound: 4096, require_pricing: true, require_current_pricing: true }
    MODEL "niffy-reasoning" (reasoning = true, effort = "medium")
    ALGORITHM static
    PLUGIN router_replay {
      enabled: true
      max_records: 1000
      capture_request_body: false
      capture_response_body: false
    }
  }

  ROUTE code-tool-route (description = "Select a tool-capable coding model; the caller remains responsible for authorizing and executing repository or terminal operations.") {
    PRIORITY 300
    WHEN keyword("code_tool_task")
    REQUIRES ["chat", "code", "tool_calling"]
    BUDGET { currency: "USD", max_estimated_cost: 0.03, output_token_bound: 3072, reasoning_token_bound: 0, require_pricing: true, require_current_pricing: true }
    MODEL "niffy-general" (reasoning = false)
    ALGORITHM static
    PLUGIN router_replay {
      enabled: true
      max_records: 1000
      capture_request_body: false
      capture_response_body: false
    }
  }

  ROUTE code-general-route (description = "Route ordinary implementation and refactoring to the general coding tier.") {
    PRIORITY 200
    WHEN keyword("code_general_task")
    REQUIRES ["chat", "code"]
    BUDGET { currency: "USD", max_estimated_cost: 0.03, output_token_bound: 3072, reasoning_token_bound: 0, require_pricing: true, require_current_pricing: true }
    MODEL "niffy-general" (reasoning = false)
    ALGORITHM static
    PLUGIN router_replay {
      enabled: true
      max_records: 1000
      capture_request_body: false
      capture_response_body: false
    }
  }

  ROUTE code-economical-route (description = "Handle simple explanations and bounded code transformations economically.") {
    PRIORITY 100
    REQUIRES ["chat", "code"]
    BUDGET { currency: "USD", max_estimated_cost: 0.01, output_token_bound: 1536, reasoning_token_bound: 0, require_pricing: true, require_current_pricing: true }
    MODEL "niffy-cheap" (reasoning = false)
    ALGORITHM static
    PLUGIN router_replay {
      enabled: true
      max_records: 1000
      capture_request_body: false
      capture_response_body: false
    }
  }
}
