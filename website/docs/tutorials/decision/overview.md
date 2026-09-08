# Decisions

## Overview

Signals tell the Router what it detected. Decisions turn those detections into
a route policy:

- which route matched
- which models are candidates
- whether reasoning is enabled
- which plugins run after the route is chosen

## Key Advantages

- Keeps route policy readable even when multiple signals must cooperate.
- Makes boolean logic explicit and reviewable.
- Separates route matching from deployment bindings, algorithms, and plugins.

## What Problem Does It Solve?

Without a decision layer, signal outputs do not tell the router how to react. Teams end up scattering route logic across ad hoc if-statements, model defaults, and plugin wiring.

Decisions solve that by turning named signals into clear route policies with stable priorities and candidate models.

## When to Use

Use a decision when:

- a route should activate from one or more signals
- the same model policy should be reused across several signal combinations
- route priority matters
- plugins or algorithms should attach to a matched route instead of the whole router

## Configuration

In v0.3, decisions live under `routing.decisions`:

```yaml
routing:
  decisions:
    - name: business_route
      description: Route business requests to the business model.
      priority: 110
      rules:
        operator: AND
        conditions:
          - type: domain
            name: business
      required_capabilities: [chat]
      request_budget:
        currency: USD
        max_estimated_cost: 0.01
        output_token_bound: 2048
        require_pricing: true
      modelRefs:
        - model: qwen2.5:3b
          use_reasoning: false
```

Classifier failures evaluate as `Unknown`, not `False`. `NOT Unknown` remains
`Unknown`; `False AND Unknown` is `False`, and `True OR Unknown` is `True`.
When the final result is still unknown, `rules.on_unknown` chooses `no_match`,
`match`, or `fail_request`. If omitted, existing generic-classifier
`on_error` and prompt-guard `on_error` behavior is retained.

Decision matching stays separate from:

- `providers.models[]`, which carries deployment bindings
- `decision.algorithm`, which chooses among multiple candidate models
- `decision.plugins`, which post-processes a matched route

After a decision matches, `required_capabilities` removes models that do not
advertise every required capability in `routing.modelCards`. `code` is the
provider-neutral hard requirement for coding profiles. This hard gate
runs before an algorithm ranks the remaining candidates. In DSL, the same
contract is `REQUIRES ["chat", "reasoning"]`. Omit it when a route has no hard
capability requirement.

`request_budget` is the next hard gate. It estimates each remaining candidate
from the request input-token estimate and explicit output/reasoning ceilings,
falling back to the configured token bounds when the caller omits them.
Candidates with missing prices (when required), stale provenance (when
required), another currency, or an estimate above `max_estimated_cost` are
removed before ranking. The DSL form is `BUDGET { currency: "USD", ... }`.

`workflow` is a distinct post-selection execution contract, not a model
capability. `web_search_answer` checks the trusted user-groups header, performs
one bounded SearXNG query, validates result URLs, and injects delimited,
provenance-tagged untrusted evidence for the selected synthesis model. The DSL
form is `WORKFLOW { type: "web_search_answer", ... }`. The route-evaluation API
shows the planned workflow with `executes_tools: false`; it never performs the
search during inspection.

`mcp_tool_call` uses the same trusted-group boundary for one configured HTTP
MCP server and tool. Discovery must return the exact tool with
`readOnlyHint=true`; destructive or non-text results fail closed. Arguments
may contain request substitutions such as `{{user_content}}`. The
dry-run response names the server and tool but does not resolve credentials,
discover capabilities, or execute the call.

Choose the smallest shape that expresses the policy clearly:

| Decision shape | Best for | Guide |
|----------------|----------|-------|
| Single condition | One decisive signal | [Single Condition](./single) |
| `AND` | Several conditions that must all match | [AND Decisions](./and) |
| `OR` | One route shared by several alternative conditions | [OR Decisions](./or) |
| `NOT` | An explicit exclusion or safety guard | [NOT Decisions](./not) |
| Composite | Nested combinations of `AND`, `OR`, and `NOT` | [Composite Decisions](./composite) |
| Retention directives | Cache or session side effects after a decision matches | [Retention Directives](./retention) |

Add [Algorithm](../algorithm/overview) when `modelRefs` contains more than one candidate, and add [Plugin](../plugin/overview) when the route needs post-selection behavior.

## Operational Boundaries

- Every leaf must reference a signal or projection output declared in the same
  recipe.
- Higher `priority` wins when more than one decision matches. Keep an explicit
  unconditional fallback or configure `providers.defaults.default_model`.
- Decision names and route diagnostics can become operational metadata; avoid
  secrets or personal identifiers in names and descriptions.
- Boolean logic is policy, not authentication. Use trusted identity through
  the `authz` service and signal for access-sensitive routes.
