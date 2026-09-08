# Niffy: Product-Independent Cost-Aware Routing

Status: active design proposal

## Purpose

Niffy is a product-independent control plane that decides how an AI request
should be handled before a provider incurs generation cost. A chat product,
agent, API service, or batch worker should be able to submit an OpenAI-style
request and receive either a routing decision or an executed response without
embedding provider-specific policy in the calling product.

The north-star outcome is not merely “pick one of three models.” Niffy must
separate task understanding, workflow selection, model eligibility, economic
selection, execution, and evidence so each layer can change independently.

## Current Baseline

The development profile under `config/niffy/` currently provides:

- one request-facing alias, `niffy/auto`;
- an economical Mistral fallback for routine prompts;
- an xAI route for explicit freshness, search, and tool intent;
- an OpenAI route for deliberate reasoning;
- deterministic keyword overrides plus calibrated local embedding and
  prototype-complexity signals;
- strict provider response normalization and live three-provider forwarding;
- provider-neutral capability eligibility and conservative request-cost
  budgets before ranking;
- an inspectable, non-executing routing-decision contract; and
- authorized, bounded SearXNG and read-only MCP workflow contracts; and
- content-free execution evidence for actual usage, cost, savings, latency,
  retries, fallback, and measured-or-explicitly-unmeasured quality.

This proves provider-independent ingress, capability and cost gates, dry-run
inspection, bounded workflow execution, availability fallback, locally
classified semantic routing, and runtime accounting. It does not yet enforce
persisted tenant budgets, learn online from outcomes, automate quality-based
escalation, or expose a stable product-facing API.

## Product Boundary

Niffy owns:

- normalization of supported request formats;
- extraction of routing signals and request facts;
- classification of task, capabilities, complexity, freshness, and risk;
- selection of an eligible workflow and model;
- budget, latency, reliability, and provider-policy enforcement;
- bounded fallback and escalation policy;
- routing explanations, accounting, and evaluation evidence.

Niffy does not own:

- the calling product's user experience or conversation database;
- unrestricted external side effects;
- provider billing contracts or credential issuance;
- business authorization that the calling product has not delegated;
- permanent storage of raw prompts or outputs by default;
- a claim that model selection alone performs web search or MCP execution.

## Decision Pipeline

The staging layer evaluates a request in this order:

1. **Normalize** the request without losing tool, response-format, context, or
   tenant-policy facts.
2. **Apply safety and authorization constraints** that can exclude workflows,
   providers, regions, or models.
3. **Classify the task** and identify required capabilities such as reasoning,
   structured output, freshness, retrieval, or tool calling.
4. **Choose an execution kind**: direct model, reasoning model, retrieval
   workflow, web-search workflow, MCP workflow, review, or rejection.
5. **Build the eligible candidate set** from capability and policy metadata.
6. **Rank candidates** by expected quality, price, latency, and reliability.
7. **Return a decision** in dry-run mode or execute it in forwarding mode.
8. **Validate the outcome**, perform only bounded escalation, and record
   privacy-safe evidence.

Tool selection must precede model selection. “Find today's exchange rate” is a
workflow request whose synthesis model is a secondary decision, not simply a
reason to choose a model marketed as having search capabilities.

## Versioned Decision Contract

The stable contract is exposed by `POST /v1/route/evaluate`; the original
`POST /api/v1/route/evaluate` path remains pinned to `v1alpha1`. It performs no
provider generation and returns a privacy-minimized envelope:

```json
{
  "schema_version": "vllm-sr/routing-decision/v1",
  "dry_run": true,
  "route": {
    "recipe": "default",
    "decision": "reasoning-route",
    "algorithm": "static"
  },
  "selection": {
    "status": "selected",
    "method": "single",
    "selected_model": "niffy-reasoning",
    "candidate_models": ["niffy-reasoning"]
  },
  "signals": {
    "matched": {"keywords": ["reasoning_intent"]}
  }
}
```

The contract intentionally omits the raw prompt and conversation. Optional
decision traces contain rule structure and signal names, not private request
content. Additive fields may be introduced during `v1alpha1`; incompatible
changes require a new schema version.

The response now includes provider-neutral capability eligibility: catalogue
version, required capabilities, eligible candidates, and bounded exclusion
reasons. Future revisions will add normalized task, workflow, policy
disposition, confidence, cost estimate, latency estimate, and fallback plan.
Fields must not claim evidence the router did not actually compute.

### Dashboard inspection surface

The dashboard exposes the same contract through **Build → Outcomes → Route
Inspector**. The inspector accepts a routing model and prompt, optionally asks
for a privacy-safe trace, and displays the selected decision, candidate model,
algorithm, explanation, and named signals. Access requires `evaluation.run`.
The dashboard proxy permits only `POST` for this route, replaces browser
credentials with the managed Router identity, and does not turn inspection into
provider or tool execution.

## Capability and Provider Abstraction

Logical model entries describe capabilities independently of provider and
provider model ID. Hard requirements use
`vllm-sr/model-capability-catalog/v1alpha1` with these canonical IDs:

- `chat`, `text`, and `reasoning`;
- `tool_calling`, `parallel_tool_calling`, `structured_output`, and
  `json_schema`;
- `vision`, `audio`, `video`, and `file`;
- `embeddings` and `image_generation`.

Context-window limits remain numeric model-card metadata and are evaluated in
the same eligibility stage. Region, data-handling, reliability, and price are
separate policy/ranking dimensions rather than capability names. Model cards
may retain additional descriptive values for compatibility, but a decision's
`required_capabilities` accepts only the canonical vocabulary. A missing model
card or missing capability fails closed only when the decision declares a hard
requirement; routes without requirements preserve existing behavior.

Eligibility is a hard filter. Ranking must never choose a cheap model that
lacks a required capability. Provider adapters remain responsible for wire
format, authentication, endpoint paths, and response normalization.

## Workflow Model

The initial workflow classes are:

- `direct-answer` for ordinary generation;
- `reasoning-answer` for deliberate analysis;
- `web-search-answer` for fresh public information;
- `retrieval-answer` for an authorized knowledge source;
- `mcp-tool-call` for an explicitly allowed MCP capability;
- `review-required` for policy-controlled human review;
- `reject` when execution is prohibited.

A workflow declares required tools, allowed side effects, evidence handling,
timeouts, retry limits, and eligible synthesis models. Search and retrieval
results are untrusted inputs and must be delimited, size-bounded, provenance
tagged, and protected against instruction injection.

The implemented first workflow contract is
`vllm-sr/workflow/v1alpha1`/`web_search_answer`. It uses a SearXNG-compatible
adapter and requires membership in an operator-trusted authorization group.
The runtime performs one GET search only after routing, model eligibility,
budget, rate-limit, and cache checks. Query characters, timeout, result count,
response bytes, and injected evidence characters are all bounded. Result URLs
must be absolute HTTP(S) URLs; normalized evidence is delimited and explicitly
marked untrusted before it is injected as a tool result for synthesis. Dry-run
reports the workflow but performs neither authorization nor tool execution.

The same contract now supports `mcp_tool_call` for a single explicitly
allowlisted tool on an operator-configured HTTP MCP server. Authorization is
checked before credentials are resolved or capability discovery begins. The
runtime discovers the configured tool, requires an explicit
`readOnlyHint=true` and no destructive hint, validates its required top-level
arguments, and performs one call within response-byte, timeout, and injected
character bounds. Only text results are accepted; they are escaped,
provenance-tagged with server and tool names, marked untrusted, and injected as
a tool result. Stdio servers and prompt-selected tool names are intentionally
outside this first boundary.

## Cost and Utility Model

For every candidate, Niffy should estimate:

```text
estimated cost =
  uncached input tokens × input price
  + cached input tokens × cached-input price
  + visible output tokens × output price
  + reasoning tokens × reasoning price
  + tool and retrieval charges
```

Candidate ranking should optimize expected utility subject to hard policy:

```text
utility = expected quality
          - cost penalty
          - latency penalty
          - reliability penalty
          - escalation-risk penalty
```

Prices are versioned configuration inputs with currency, unit, source, and
effective date. Estimated and provider-reported actual cost must remain
separate. The main savings metric is actual routed cost compared with a named
premium-only baseline, including router, retry, and tool overhead.

The first implemented contract is
`vllm-sr/provider-pricing/v1alpha1`. Provider entries pin source, version,
`per_1m_tokens` unit, effective time, and expiry. A decision's
`request_budget` supplies the currency, maximum estimated cost, and default
output/reasoning bounds. Explicit request bounds replace those defaults.
Missing, stale, wrong-currency, and over-budget candidates are excluded before
ranking, and dry-run exposes all alternative estimates without executing a
provider.

## Policy and Tenancy

Policy is evaluated independently from classification. It should support:

- per-request and period budgets;
- tenant-specific eligible models and providers;
- maximum latency and minimum quality tier;
- geographic and data-residency constraints;
- whether external tools or side effects are allowed;
- maximum escalation count and cost;
- provider outage and circuit-breaker behavior;
- explicit model pinning for authorized callers.

Requests must carry authenticated tenant and policy context through trusted
metadata or headers. Prompt text must never be allowed to grant itself a
larger budget or broader tool authority.

## Conversation Stability

Multi-turn routing needs a defined scope. Niffy should distinguish request,
conversation, and session identity. A protected conversation may remain on a
model for tool loops and cache continuity, while an explicit capability or
policy change may authorize a switch. Every sticky decision needs an expiry,
reason, and bypass rule.

## Fallback and Escalation

Fallback handles availability; escalation handles insufficient capability or
validated quality. They must be distinguishable in telemetry.

Allowed triggers include timeout, rate limit, provider outage, invalid schema,
context overflow, tool failure, low confidence, or an explicit quality gate.
Every chain requires maximum attempts, maximum incremental cost, allowed
providers, and a terminal disposition. Niffy must not routinely call an
expensive model after every economical response, because that destroys the
economic objective.

NIFFY-07 implements availability fallback as an ordered Looper algorithm. Its
failure classes and model-attempt count are explicit; non-retryable failures
stop the chain. Runtime model circuits use configured consecutive-failure and
open-duration bounds, and health-open models are excluded before selection.
Pre-execution estimates multiply each model by its same-model Envoy retry
allowance and admit only the prefix of a fallback chain that fits the decision
budget. Dry-run returns `vllm-sr/resilience-plan/v1alpha1` and never calls or
probes a provider. Automated response-quality escalation remains separate from
availability fallback and requires a configured quality gate.

## Privacy and Security

- Provider keys remain environment- or secret-store-backed and are redacted
  from logs, APIs, traces, and tracked files.
- Dry-run decisions do not echo prompts, messages, tool arguments, schemas, or
  retrieved content.
- Raw traffic capture is disabled by default and requires scoped consent,
  encryption, retention, deletion, and audit policy.
- Evaluation datasets derived from traffic are minimized and anonymized.
- Tool execution uses explicit allowlists, bounded input/output, timeouts, and
  per-tool authorization.
- Provider and tool output is treated as untrusted data.

## Observability and Accounting

Each executed request should produce a correlation ID and privacy-safe record
of decision, workflow, selected model, candidate count, policy disposition,
estimated cost, actual usage, latency, retries, escalation, cache behavior,
and terminal status. Model and provider health should be measured separately.

Metrics must support cost per successful request, routing distribution,
premium-model avoidance, fallback rate, escalation rate, decision latency,
provider latency, quality-gate failure, and savings against the configured
baseline. Cardinality must be bounded; tenant IDs and request content do not
belong in metric labels.

NIFFY-08 records `vllm-sr/execution-evidence/v1alpha1` as a content-free
Router Replay outcome. It correlates provider-reported token usage with pinned
pricing, the configured premium baseline, observed end-to-end latency, Envoy
same-model attempt counts, Looper model attempts, and fallback use. The replay
aggregate exposes totals for these fields, while Prometheus uses only bounded
algorithm, retry-kind, quality-state, and currency labels. Request and response
capture remain disabled in the Niffy profile. Quality is recorded as
`not_measured` unless an enabled quality guard actually evaluates it.

NIFFY-09 keeps keyword matches as deterministic policy overrides and adds two
provider-independent local signals: `live_information_task` uses embedding
similarity for freshness/search paraphrases, and `reasoning_demand` uses a
hard-versus-easy prototype margin. Both are evaluated without a provider LLM
call. Their thresholds, prototype text, contract version, and tagged probes are
versioned inputs to calibration; tool intent retains higher priority than
reasoning when both match.

NIFFY-10 promotes the non-generating decision envelope to a stable v1 contract,
adds a small Python client that rejects incompatible schemas, and enforces a
versioned tenant policy before capability and cost ranking in both dry-run and
live routing. Tenant rules can allow or deny logical models and providers and
apply a stricter per-request cost ceiling. Identity is read only from a trusted
header and is reduced to a boolean presence signal in public evidence. The
Niffy profile also moves Router Replay and startup status to Redis so those
records survive router-container restarts.

NIFFY-12 adds the isolated `niffy/code` entrypoint. Its recipe classifies
ordinary implementation, repository/tool intent, complex coding demand, and
large context independently from general chat policy. The provider-neutral
capability catalogue now permits `code` as a hard requirement, allowing all
three economic tiers to advertise coding fitness without encoding a provider
name in a decision. Tool-capable routing never grants execution authority.

Coding retry escalation requires two independent inputs: an objective
`patch_failed`, `build_failed`, `test_failed`, or `review_failed` status in
request metadata and the trusted `code_validator` role injected for an
authenticated coding orchestrator. Prompt text and untrusted metadata alone
cannot select this escalation path. Offline promotion uses the versioned
`niffy/code-evaluation/v1` ledger to verify each escalation against the
previous attempt's failed check and gates total trajectory cost per validated
outcome, validation rate, and savings against a premium-only baseline.

## Evaluation Strategy

Development uses four complementary suites:

1. **Routing fidelity:** expected decision, workflow, signals, and model.
2. **Boundary robustness:** ambiguous language, collisions, multi-turn input,
   adversarial prompts, and negative examples.
3. **Economic evaluation:** quality and success held against cost, latency,
   retries, and the premium-only baseline.
4. **Failure evaluation:** provider outages, malformed responses, exhausted
   budgets, tool failures, and deterministic fallback behavior.

Promotion requires explicit pass rates and complete cost ledgers. Reachability
or “at least one success” is not adequate evidence.

## Rollout Strategy

New decision logic progresses through offline probes, live dry-run shadowing,
bounded canary execution, and explicit promotion. Shadow decisions must not
call providers or tools. Each rollout pins the policy, catalogue, price data,
classifier, and evaluation version so results remain reproducible.

## Major Risks

- Router overhead can erase savings for small requests.
- A separate LLM used for every routing decision can cost more than it saves.
- Provider marketing labels are not reliable capability evidence.
- Keyword-only classification has brittle recall and collision behavior.
- Unbounded retries and escalation silently multiply cost.
- Tool intent without actual tool execution produces misleading answers.
- Price drift makes historical savings incomparable without versioned prices.
- Storing prompts for optimization creates privacy and compliance exposure.
- Provider-specific response differences can break an otherwise correct route.

## Delivery Milestones

1. **Inspectable staging decision:** versioned, non-forwarding endpoint and
   dashboard inspector with privacy-safe explanations and exact routing parity.
2. **Capability catalogue:** eligibility filtering independent of provider.
3. **Cost estimation (implemented):** versioned prices, token estimates,
   request budgets, and alternative-cost comparison.
4. **Workflow routing (implemented):** authorized bounded SearXNG evidence and
   allowlisted read-only HTTP MCP execution are available.
5. **Resilience:** health-aware fallback, bounded escalation, and circuit
   breaking.
6. **Evidence (implemented):** usage accounting, savings telemetry, explicit
   quality outcomes, retries, latency, and replayable execution evidence.
7. **Adaptive routing (local classifiers implemented):** calibrated local
   classifiers retain deterministic overrides; online learning from
   privacy-safe outcomes remains future work.
8. **Product contract (implemented):** stable SDK-facing API, tenant policy,
   compatibility aliasing, and Redis-backed development deployment hardening.

## Open Decisions

- Which automated review/update process should replace the initial pinned
  official provider sources before their configured expiry.
- Whether workflow execution belongs in this process or a separately isolated
  worker.
- Which quality signals are cheap and reliable enough for online escalation.
- How tenant quotas are persisted and reconciled across replicas.

Execution state, stable task IDs, and the single current next action live in
[PL-0041: Niffy Cost-Aware Router](../../../tools/agent/docs/plans/pl-0041-niffy-cost-aware-router.md).
