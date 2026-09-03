# Niffy development profile

`config.yaml` is the first cost-oriented Niffy routing profile. It exposes
one OpenAI-compatible endpoint through the `niffy/auto` model name and maps
requests to three logical model tiers:

| Decision | Provider model | Purpose |
| --- | --- | --- |
| `tool-intent-route` | xAI `grok-4.3` | Search, freshness, and tool-intent prompts |
| `reasoning-route` | OpenAI `gpt-5.6-terra` | Complex analysis and deliberate reasoning |
| `economical-default-route` | Mistral `mistral-small-latest` | Routine prompts and the fallback route |

The profile never stores credentials. It reads `OPENAI_API_KEY`, `XAI_API_KEY`,
and `MISTRAL_API_KEY` from the process environment. From the repository root:

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

The durable product design and active build sequence are tracked in:

- [Niffy Product-Independent Cost-Aware Routing](../../website/docs/proposals/niffy-cost-aware-routing.md)
- [PL-0041: Niffy Cost-Aware Router](../../tools/agent/docs/plans/pl-0041-niffy-cost-aware-router.md)

Selecting `tool-intent-route` does not by itself execute web search or an MCP
tool. It selects the model assigned to that class of request. Tool discovery,
permission policy, execution, and result injection are the next integration
layer.

Provider prices are configuration inputs for cost-aware routing and reporting;
they should be reviewed when providers change their prices.
