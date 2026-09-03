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
- [ ] `NIFFY-03` Define the capability catalogue and hard eligibility filter.
- [ ] `NIFFY-04` Add versioned provider pricing, request cost estimation,
  request budgets, and alternative-cost comparison.
- [ ] `NIFFY-05` Define workflow contracts and implement an authorized
  web-search workflow.
- [ ] `NIFFY-06` Add generic MCP discovery, authorization, execution, and
  result-injection boundaries.
- [ ] `NIFFY-07` Add health-aware fallback, bounded escalation, and provider
  circuit breaking.
- [ ] `NIFFY-08` Record privacy-safe usage, actual cost, latency, quality,
  retries, and savings evidence.
- [ ] `NIFFY-09` Introduce calibrated local task/capability classifiers while
  retaining deterministic policy overrides.
- [ ] `NIFFY-10` Stabilize the product-facing API and complete tenant policy,
  SDK, deployment, and compatibility hardening.

## Next Action

Complete `NIFFY-03`: define a provider-independent capability vocabulary and
catalogue schema, then apply hard capability eligibility before the existing
model-selection algorithm ranks candidates.

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
