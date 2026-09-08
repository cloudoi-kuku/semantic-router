# PL-0041: Niffy Cost-Aware Router

## Goal

Turn the validated three-provider Niffy profile into a product-independent
staging router that can explain, price, execute, and evaluate model and tool
workflow decisions.

## Scope

- A versioned, privacy-minimized routing-decision contract.
- Provider-independent capability and pricing metadata.
- Tool/workflow selection before synthesis-model selection.
- Tenant budget and provider policy.
- Bounded fallback, escalation, accounting, and evaluation.
- OpenAI-compatible integration for calling products.

## Non-Goals

- Owning product UI or conversation persistence.
- Granting tool authority from prompt content.
- Persisting raw prompts or outputs by default.
- Using an expensive LLM for every routing decision.
- Claiming search or MCP execution before a workflow actually runs.

## Exit Criteria

- Calling products can request a dry-run decision or an executed response
  through versioned contracts without provider-specific routing logic.
- Capability and policy constraints determine eligibility before economic
  ranking.
- Search and MCP requests select authorized workflows, not merely a model.
- Every execution has bounded retry/escalation and complete cost accounting.
- Evaluation demonstrates routing fidelity, quality, latency, resilience, and
  savings against a pinned premium-only baseline.
- Credentials and private request content remain outside tracked artifacts and
  public telemetry.

## Task List

- [x] `NIFFY-01` Validate direct OpenAI, xAI, and Mistral routing with a
  deterministic three-decision profile and calibration probes.
- [x] `NIFFY-02` Ship `vllm-sr/routing-decision/v1alpha1` at
  `POST /api/v1/route/evaluate` and expose it through a permission-controlled
  dashboard inspector, with no provider generation or raw prompt echo.
- [x] `NIFFY-03` Define the provider-neutral
  `vllm-sr/model-capability-catalog/v1alpha1` vocabulary, carry hard
  `required_capabilities` through YAML/DSL/CLI/dashboard contracts, filter
  candidates before live and dry-run ranking, preserve the gate during Router
  Learning expansion, and expose privacy-safe eligibility evidence.
- [x] `NIFFY-04` Add `vllm-sr/provider-pricing/v1alpha1` provenance and
  freshness metadata, conservative input/output/reasoning cost estimates,
  hard per-request decision budgets, and inspectable alternative-cost
  comparison before ranking.
- [x] `NIFFY-05` Define `vllm-sr/workflow/v1alpha1` and implement one
  authorized, bounded SearXNG web-search workflow with untrusted,
  provenance-tagged evidence injection and a non-executing dry-run preview.
- [x] `NIFFY-06` Add generic MCP discovery, authorization, execution, and
  result-injection boundaries.
- [x] `NIFFY-07` Add health-aware fallback, bounded escalation, and provider
  circuit breaking.
- [x] `NIFFY-08` Record `vllm-sr/execution-evidence/v1alpha1` privacy-safe
  usage, actual cost, latency, explicit quality status, retries, fallback, and
  savings evidence in Router Replay and bounded-cardinality metrics.
- [x] `NIFFY-09` Introduce calibrated local embedding and prototype-complexity
  classifiers while retaining deterministic keyword policy overrides and tool
  priority.
- [x] `NIFFY-10` Stabilize the product-facing API at
  `POST /v1/route/evaluate`, retain the `v1alpha1` compatibility path, enforce
  privacy-safe tenant model/provider/cost policy in live and dry-run routing,
  ship a schema-checking Python client, and move the Niffy replay/startup state
  to Redis-backed development persistence.
- [x] `NIFFY-11` Package the extended router and dashboard as immutable,
  versioned images; record content-addressed image and config evidence; provide
  a no-pull verified startup command; and pass clean-start, restart-persistence,
  rollback, security-boundary, and full feature-gate validation.
- [x] `NIFFY-12` Define and calibrate a product-independent `niffy/code`
  profile that classifies coding intent, complexity, context and tool needs;
  routes among economical, general, and reasoning coding tiers; and escalates
  only from objective patch, build, test, or review evidence.
- [ ] `NIFFY-13` Run a bounded coding canary across all three configured
  providers, collect real `niffy/code-evaluation/v1` outcome ledgers, and tune
  routing thresholds against validated quality, latency, and cost evidence.

## Next Action

Restore valid xAI credentials and Mistral quota, then run the NIFFY-13 bounded
coding canary and promote thresholds only if the real outcome ledger passes the
cost-per-validated-outcome gate.

## Operating Rules

- Keep classification, workflow selection, model eligibility, and economic
  ranking as distinct stages.
- Apply hard safety, authorization, capability, region, and budget constraints
  before utility ranking.
- Do not echo raw request content from decision or telemetry APIs.
- Do not execute providers or tools in dry-run mode.
- Version price, policy, catalogue, classifier, and evaluation inputs.
- Bound every retry, escalation, tool call, and incremental cost.
- Promote behavior only with explicit routing, quality, cost, latency, and
  failure thresholds.
- Keep this plan limited to active execution state; durable rationale belongs
  in the linked product proposal.

## Related Docs

- [Niffy Product-Independent Cost-Aware Routing](../../../../website/docs/proposals/niffy-cost-aware-routing.md)
- [Niffy development profile](../../../../config/niffy/README.md)
- [Evaluation Plane](../../../../website/docs/benchmarking/evaluation-plane.md)
- [Testing strategy](../testing-strategy.md)
