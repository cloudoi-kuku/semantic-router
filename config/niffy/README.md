# Niffy development profile

`config.yaml` is the first cost-oriented Niffy routing profile. It exposes
one OpenAI-compatible endpoint through the `niffy/auto` model name and maps
requests to three logical model tiers:

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

Applications can then send their existing OpenAI-style request to the router:

```bash
curl http://localhost:8899/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "niffy/auto",
    "messages": [{"role": "user", "content": "Explain DNS in one paragraph."}]
  }'
```

To inspect the route without generating a provider response, open **Build →
Outcomes → Route Inspector** in the dashboard. The equivalent direct Router API
request is:

```bash
curl 'http://localhost:8080/api/v1/route/evaluate?trace=true' \
  -H 'Content-Type: application/json' \
  -d '{"model":"niffy/auto","text":"Analyze the trade-offs of this design."}'
```

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
`execution_evidence`. The development memory store expires records after seven
days and is not durable across restarts.

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
