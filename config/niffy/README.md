# Niffy development profile

`config.yaml` is the cost-oriented Niffy routing profile. It exposes the
general `niffy/auto` entrypoint and the isolated `niffy/code` coding entrypoint,
then maps requests to three logical model tiers:

| Decision | Provider model | Purpose |
| --- | --- | --- |
| `tool-intent-route` | SearXNG + xAI `grok-4.3` | Authorized web evidence followed by synthesis |
| `reasoning-route` | OpenAI `gpt-5.6-terra` | Complex analysis and deliberate reasoning |
| `economical-default-route` | Mistral `mistral-small-latest` | Routine prompts and the fallback route |

The profile never stores credentials. It reads `OPENAI_API_KEY`, `XAI_API_KEY`,
and `MISTRAL_API_KEY` from the process environment. The optional
`NIFFY_WEB_SEARCH_ENDPOINT` overrides the development SearXNG endpoint. From
the repository root:

```bash
set -a
source .env
set +a
.venv-agent/bin/vllm-sr validate --config config/niffy/config.yaml
```

Start the minimal local router stack with the same exported environment:

```bash
.venv-agent/bin/vllm-sr serve \
  --config config/niffy/config.yaml \
  --minimal
```

That command is convenient while editing but is not the NIFFY-11 release
path. Build content-addressed, version-tagged artifacts and generate their
local release manifest with:

```bash
make niffy-release NIFFY_ALLOW_DIRTY=1
```

`NIFFY_ALLOW_DIRTY=1` is only for validating an uncommitted development tree;
omit it for a reviewed release. The supported start command verifies that the
config and local image IDs still match the manifest and refuses to pull a
replacement image:

```bash
make niffy-release-start
```

The verified release starts in minimal mode by default so the dashboard does
not receive the local Docker socket. Use `NIFFY_MINIMAL=0` only for a trusted
local dashboard and observability session. A release build defaults to
`niffy/router:v0.4.0` and
`niffy/dashboard:v0.4.0`; the registry and tag can be overridden with
`NIFFY_REGISTRY` and `NIFFY_TAG`. Published environments should retain the
manifest's image IDs or replace the local references with registry digest
references after pushing. The manifest also records the exact path, size, and
SHA-256 digest of every external classifier/embedding model file. Seed the
release model cache from a trusted artifact copy before starting on another
host; startup verification rejects missing or changed weights.

Keep a copy of each published manifest with the release artifacts. Rollback is
the same verified start operation with the previous immutable tag, manifest,
and matching configuration:

```bash
make niffy-release-start \
  NIFFY_TAG=v0.2.0 \
  NIFFY_MANIFEST=/path/to/niffy-v0.2.0-release-manifest.json
```

The bundled profile is safe for a loopback development host, not direct public
Internet exposure. The management endpoint remains host-loopback-only and the
verified start defaults to the minimal stack. A production gateway must
authenticate inference callers, strip caller-provided identity headers, inject
the trusted tenant identity, terminate TLS, enforce rate limits, and keep the
management API on a private authenticated control-plane network. If the local
dashboard is enabled with `NIFFY_MINIMAL=0`, it receives the Docker socket and
must be treated as host-administrator access.

Applications can then send their existing OpenAI-style request to the router:

```bash
curl http://localhost:8899/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "niffy/auto",
    "messages": [{"role": "user", "content": "Explain DNS in one paragraph."}]
  }'
```

Use `niffy/code` when the calling product already knows the interaction is a
coding workload. This avoids mixing broad chat heuristics with coding policy:

```bash
curl http://localhost:8899/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "niffy/code",
    "messages": [{"role": "user", "content": "Write a unit test for this parser."}]
  }'
```

The coding recipe has five ordered decisions:

| Decision | Tier | Trigger |
| --- | --- | --- |
| `code-evidence-escalation-route` | reasoning | Trusted orchestrator role plus a failed patch, build, test, or review status |
| `code-reasoning-route` | reasoning | Complex/security/concurrency/migration intent or at least 12K estimated input tokens |
| `code-tool-route` | general | Repository, terminal, patch, build, test, or MCP intent |
| `code-general-route` | general | Ordinary implementation, refactoring, bug-fix, and unit-test generation |
| `code-economical-route` | economical | Simple explanation or bounded transformation fallback |

`code-tool-route` only selects a model that can emit tool calls. Niffy does not
grant filesystem, terminal, source-control, or MCP authority. The calling
product must provide its own sandbox, allowlist, approval, and execution loop.

Premium retry escalation is deliberately not inferred from prompt text. A
gateway must strip caller-provided identity headers and inject both a verified
user ID and membership in `code-validation-producers`. The coding orchestrator
then supplies one of `patch_failed`, `build_failed`, `test_failed`, or
`review_failed` as the `niffy.code.validation` request metadata value. The
metadata or trusted role alone is insufficient to match the escalation route.
The `niffy-economy` tenant policy can still deny the reasoning model.

Routing fidelity is versioned in `probes.yaml`. Release promotion additionally
uses an objective coding-task ledger and the NIFFY-12 outcome gate:

```bash
make niffy-code-eval NIFFY_CODE_LEDGER=/path/to/code-evaluation.json
```

The JSON ledger uses schema `niffy/code-evaluation/v1`. Every task declares a
premium-only `baseline_cost_usd`, required checks from `patch`, `build`, `test`,
and `review`, and an ordered `attempts` list. An attempt records its tier, actual
USD cost, trigger, and check statuses (`passed`, `failed`, or `not_run`). The
first trigger is `initial` or `classified_complex`; every later, higher-tier
attempt must use `<check>_failed` matching a failed check on the immediately
preceding attempt. Thresholds set the minimum validated-task rate, maximum
total trajectory cost per validated outcome, and minimum savings versus the
premium-only baseline. Failed trajectories remain in total cost and cannot
make the result look artificially economical.

To inspect the route without generating a provider response, open **Build →
Outcomes → Route Inspector** in the dashboard. Products should use the stable
`/v1/route/evaluate` contract; `/api/v1/route/evaluate` remains available for
the original `v1alpha1` compatibility contract:

```bash
curl 'http://localhost:8080/v1/route/evaluate?trace=true' \
  -H 'Content-Type: application/json' \
  -d '{"model":"niffy/auto","text":"Analyze the trade-offs of this design."}'
```

NIFFY-10 adds a trusted tenant-policy stage before capability and economic
ranking. The default development policy permits the three configured providers
and caps each request at 0.20 USD. A request carrying
`x-authz-tenant-id: niffy-economy` cannot use `niffy-reasoning` and has a 0.01
USD cap. Tenant identity must be stripped and injected by an authenticating
gateway in a deployed environment; it is never accepted from the JSON body or
returned in routing evidence.

Explicit keyword matches remain deterministic policy overrides. A local
embedding signal also recognizes freshness/search paraphrases, and a local
prototype-complexity signal recognizes reasoning demand without making a paid
classifier call. Their pinned thresholds and representative positive and
negative cases live in `probes.yaml`; change a threshold or prototype only with
a new 100% passing calibration report.

Each decision also declares a hard provider-neutral capability requirement.
`tool-intent-route` requires `chat` and `tool_calling`, `reasoning-route`
requires `chat` and `reasoning`, and the economical fallback requires `chat`.
The router filters candidates against the model cards before ranking. Route
Inspector reports the catalogue version, eligible models, and any bounded
exclusion reasons without echoing the prompt.

Each route also has a conservative per-request budget. The estimate uses the
local input-token estimate plus the caller's output/reasoning bounds when
present, otherwise the route defaults. Candidate prices are pinned with a
source, version, unit, effective time, and expiry. Missing, expired,
wrong-currency, or over-budget candidates fail closed before ranking. Route
Inspector shows every alternative estimate separately from provider-reported
actual cost.

The economical route executes an ordered two-provider availability chain:
Mistral first, then xAI only after an allowed transport, timeout, rate-limit,
server, or invalid-response failure. The chain is capped at two model attempts.
Each provider permits one same-model Envoy retry and opens its router-side
circuit after three consecutive retryable failures for 30 seconds. Cost
eligibility reserves the worst case—the Envoy retry allowance plus each model
reachable in the chain—before any provider call. Route Inspector displays this
plan without executing or probing either provider.

Every route enables Router Replay with request and response capture disabled.
After an executed response, the replay record receives one versioned,
content-free `router_execution` outcome containing actual provider-reported
usage, price-derived actual cost and premium-baseline savings, end-to-end
latency, model attempts, same-model retries, cross-provider fallback, and an
explicit quality state. Quality is `not_measured` unless an enabled quality
guard produces evidence; it is never inferred from a successful HTTP status.
The aggregate replay endpoint summarizes this data under
`execution_evidence`. NIFFY-10 stores replay records and shared startup status
in separate Redis databases so router-container restarts retain inspection
data. The seven-day replay TTL remains enforced. Production deployments must
provide durable Redis storage and authentication appropriate to their trust
boundary.

Python callers can use the packaged product-neutral client:

```python
from vllm_sr import SemanticRouterClient

router = SemanticRouterClient(
    "http://router.internal:8080",
    token="management-api-token",
)
decision = router.evaluate(
    {"model": "niffy/auto", "text": "Compare these designs."},
    tenant_id="verified-tenant",
)
print(decision.selection.selected_model)
```

`tool-intent-route` now uses the `vllm-sr/workflow/v1alpha1`
`web_search_answer` contract. Live requests must arrive with the trusted
`x-authz-user-groups` header containing `web-search-users`; an authenticating
gateway must strip caller-supplied identity headers and inject its verified
values. The workflow performs one bounded SearXNG-compatible request, rejects
oversized queries and responses, validates evidence URLs, and injects at most
8,000 characters of delimited evidence marked as untrusted. Route Inspector
shows the workflow and its bounds but never authorizes or executes it.

The durable product design and active build sequence are tracked in:

- [Niffy Product-Independent Cost-Aware Routing](../../website/docs/proposals/niffy-cost-aware-routing.md)
- [PL-0041: Niffy Cost-Aware Router](../../tools/agent/docs/plans/pl-0041-niffy-cost-aware-router.md)

NIFFY-06 also provides opt-in `mcp_tool_call` workflows. They are not enabled
in this default profile because an operator must choose a trusted MCP endpoint,
server identity, exact read-only tool, authorization group, and arguments. A
minimal route payload is:

```yaml
workflow:
  type: mcp_tool_call
  authorization_group: mcp-users
  mcp:
    server_name: internal-catalog
    endpoint: https://mcp.example.com
    tool_name: lookup
    arguments:
      query: "{{user_content}}"
    timeout_seconds: 5
    max_response_bytes: 262144
    max_result_characters: 8000
    require_read_only: true
```

The endpoint must support the router's HTTP MCP capability paths. Discovery
must advertise the named tool with `readOnlyHint=true`; otherwise the request
fails closed before execution.

Provider prices are configuration inputs for cost-aware routing and reporting.
The Niffy entries expire on 2026-12-03 so an unattended catalogue cannot be
treated as current indefinitely; review the official source and advance the
version/effective/expiry window before then.
