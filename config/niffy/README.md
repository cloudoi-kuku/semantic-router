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

The keyword rules are an intentionally explainable baseline. They should be
evaluated against representative prompts before adding a trained classifier or
semantic signal.

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

Generic MCP discovery and execution remain the next workflow integration.

Provider prices are configuration inputs for cost-aware routing and reporting.
The Niffy entries expire on 2026-12-03 so an unattended catalogue cannot be
treated as current indefinitely; review the official source and advance the
version/effective/expiry window before then.
