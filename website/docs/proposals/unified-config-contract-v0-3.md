---
title: Unified Config Contract v0.3
description: Records the implemented configuration contract shared by the router, CLI, dashboard, Helm, operator, and DSL.
created: 2026-03-17
status: Implemented
---

> **Status:** Implemented · **Created:** 2026-03-17

## Problem

The router, CLI, dashboard, Helm chart, operator, and DSL previously interpreted
overlapping configuration shapes. A file accepted by one surface could require
translation or undocumented defaults in another. Model identity was also mixed with
deployment endpoints and credentials.

## Implemented contract

The public configuration has seven top-level sections:

```yaml
version:
listeners:
providers:
routing:
entrypoints:
recipes:
global:
```

| Section | Responsibility |
| --- | --- |
| `version` | Selects the configuration contract. |
| `listeners` | Defines request-facing and management listeners. |
| `providers` | Binds logical model names to provider identifiers and endpoints. |
| `routing` | Defines the default model cards, signals, projections, decisions, algorithms, and plugins. |
| `entrypoints` | Maps request-facing model names to the default profile or a named recipe. |
| `recipes` | Defines additional isolated routing profiles that share providers and global infrastructure. |
| `global` | Holds router-wide services, stores, integrations, model modules, and sparse runtime overrides. |

Unknown or retired shapes should fail with a clear validation error rather than be
silently translated at runtime.

## Provider and model boundary

`providers.defaults` owns the default provider behavior and default model.
`providers.models[].backend_refs[]` owns physical backend bindings.
`providers.models[].pricing` owns optional deployment cost metadata used by
cost-aware selection and accounting. The provider-pricing v1alpha1 contract
adds `version`, `source`, the fixed `per_1m_tokens` unit, `effective_at`, and
`expires_at`; pricing does not belong to routing model cards.

`routing.modelCards` describes routing-facing model identity. Optional
`routing.modelCards[].loras` declare LoRA adapters that decisions may select with
`lora_name`. Signals and decisions reference logical model names, not endpoints or
credentials.

Decisions may declare `required_capabilities`. The router applies these as a
hard eligibility filter against `routing.modelCards[].capabilities` before
selection. The versioned v1alpha1 vocabulary is `chat`, `text`, `code`, `reasoning`,
`tool_calling`, `parallel_tool_calling`, `structured_output`, `json_schema`,
`vision`, `audio`, `video`, `file`, `embeddings`, and `image_generation`.
Additional model-card values remain
descriptive and backward compatible; only the canonical values can be used as
hard requirements.

Decisions may also declare `request_budget`. Its currency, maximum estimated
cost, and default output/reasoning token bounds form a hard pre-execution
policy. Explicit request token ceilings replace the configured defaults for
that request. `require_pricing` and `require_current_pricing` provide
fail-closed behavior before an algorithm ranks the remaining candidates.

`global.services.tenant_policy` adds router-wide, versioned tenant constraints
without placing product logic in a recipe. An authenticating gateway strips
caller-provided identity and injects the configured trusted header. Default and
tenant-specific model/provider rules are applied before capability and economic
selection; an optional maximum estimated cost tightens the decision budget and
requires pricing. Provider-facing policy identity is carried by
`provider:<vendor>` model-card tags and remains distinct from a backend's wire
protocol. Routing evidence reports only policy status and effective constraints,
never the tenant identifier.

The optional decision-level `workflow` runs after routing, eligibility, budget,
rate-limit, and cache checks but before synthesis. The first contract is
`vllm-sr/workflow/v1alpha1` with `type: web_search_answer`. It requires an
operator-trusted authorization group and a SearXNG adapter with explicit query,
timeout, result, response-byte, and evidence-character limits. Search evidence
is normalized, URL-validated, provenance tagged, marked untrusted, and injected
as a tool result. Dry-run reports the plan and never evaluates caller authority
or executes the search endpoint.

The workflow union also accepts `type: mcp_tool_call` with an `mcp` payload.
It names one HTTP server and one allowlisted tool, requires the discovered tool
to advertise `readOnlyHint=true`, validates required arguments, accepts text
results only, and enforces timeout, response-byte, and injected-character
limits. Dry-run exposes the plan without network access or secret resolution.

## Routing and DSL boundary

Routing owns:

- model cards;
- named signals and projections;
- decisions, hard `required_capabilities`, request budgets, workflows, candidate
  `modelRefs`, algorithms, and plugins;
- route-local output and adaptation policy.

Algorithms may declare `minimum_candidates` as a portable Recipe contract.
The `fallback` Looper algorithm also declares `max_attempts` and retryable
failure classes. Provider `reliability` owns runtime circuit thresholds and
open duration, while the decision request budget owns the cumulative
worst-case cost ceiling across Envoy retries and fallback attempts.
Model-free assets can carry the declaration with empty `modelRefs`; a concrete
Entrypoint binding must satisfy it, and request-time eligibility filters must
preserve it before selection or multi-model execution begins.

Structured request controls remain facts at the signal boundary. For example,
conversation signals expose whether the protocol requires or forbids tool
execution, projections reconcile those facts with text-derived observations,
and decisions consume the resulting policy-facing output.

Top-level `entrypoints` select the default routing profile or a named item from
top-level `recipes`; they are not nested inside `routing`.

The DSL is an authoring view of routing semantics. It does not own provider
credentials, listeners, stores, or global runtime services. Import and export must
preserve the same canonical routing document rather than invent another steady-state
schema.

Classifier backend failures enter decision evaluation as `Unknown`. `NOT` preserves
that state, while `AND` and `OR` use CEL-style short-circuit semantics. A decision
resolves a terminal `Unknown` with root-level
`rules.on_unknown: no_match|match|fail_request`; omission preserves the existing
per-family compatibility behavior.

## Entrypoints and multi-recipe routing

`entrypoints[]` map request model names to either top-level routing or one named
recipe. `recipes[]` contain isolated routing profiles that reuse the same provider
inventory and global runtime.

This keeps the public API stable while allowing several routing policies to coexist in
one process. An entrypoint resolves the recipe before signals and decisions run.

## Defaults and configuration source

Built-in defaults live in the router. `global.router.config_source` selects file-backed
configuration or Kubernetes CRD reconciliation. External templates must not apply
hidden defaults after validation.

Built-in category/domain inference keeps its runtime policy in
`global.model_catalog.modules.classifier.domain`. The local model uses the
canonical `variant` field; a remote classifier uses the shared `backend` block
(`protocol`, `contract`, `model`, and `deadline_ms`) and resolves `model` by
exact external-catalog name. The category consumer currently accepts
`http_classify` plus `label_distribution.v1`, preserving the full label-score
distribution. Prompt guard remains on its existing configuration surface until
its separately scoped migration.
Connector byte ceilings belong to the connector configuration. External LLM
classifier entries and the MCP classifier module use `max_response_bytes`.
The dashboard, Helm chart, and operator may help users author or transport config, but
the resulting document still uses the same contract.

## Repository sources

`config/config.yaml` is the exhaustive canonical reference config. Reusable examples
live under:

- `config/fragments/signal/`;
- `config/fragments/decision/`;
- `config/fragments/algorithm/`; and
- `config/fragments/plugin/`.

Runtime deployment examples remain separate from routing fragments. Contract tests
and `make agent-lint` keep the reference config, schema, examples, and public docs
aligned.

## Migration

Use `vllm-sr config migrate --config old-config.yaml` to convert supported legacy
layouts. Review the result, resolve credentials through the deployment's secret
mechanism, and validate it before serving.

`vllm-sr init` was removed. Canonical YAML is the steady-state configuration source;
interactive or graphical authoring tools must export that same document.

## Scope and non-goals

The contract unifies configuration ownership. It does not require every authoring
surface to expose every advanced field in one form, nor does it make the DSL a
deployment-language replacement.

## References

- [Current configuration guide](../installation/configuration)
- [Configuration workflows](../installation/configuration-workflows)
- [Signals, decisions, and model selection](../overview/signal-driven-decisions)
- [Virtual Models](../tutorials/global/entrypoints-and-recipes)
- [Related issue #1505](https://github.com/vllm-project/semantic-router/issues/1505)
