---
title: Tenant Routing Policy
description: Apply trusted tenant model, provider, and per-request cost constraints before routing.
---

# Tenant Routing Policy

## Overview

`global.services.tenant_policy` lets multiple products share one router while
keeping product and provider SDK logic outside the routing core. It is a hard
eligibility stage: tenant constraints run before capability checks and
cost-aware ranking in both live traffic and `POST /v1/route/evaluate` dry runs.

## Key Advantages

- Keeps tenant policy independent of any calling chat or application product.
- Prevents disallowed or over-budget models from reaching economic ranking.
- Preserves vendor identity when providers share a compatible API protocol.
- Returns inspectable policy evidence without exposing tenant identifiers.

## What Problem Does It Solve?

A shared router otherwise has no trusted, uniform way to prevent one product or
tenant from selecting a provider, premium model, or request cost that its policy
does not permit. This service applies those constraints consistently to dry-run
inspection and live execution.

## When to Use

Use tenant routing policy when multiple authenticated products or tenants share
the same router and need different model, provider, or per-request cost limits.
Do not treat it as a billing ledger or monthly quota system.

## Trust boundary

The router reads tenant identity only from `tenant_id_header`; it does not
accept tenant identity in the request body. Deploy an authenticating gateway in
front of the router that removes any caller-supplied copy of that header and
injects a verified value. Keep the management API authenticated or private.

Tenant identifiers are never returned by the routing-decision API or emitted as
metric labels. The response exposes only whether identity was present, which
policy source applied, and the effective non-secret constraints.

## Configuration

Tag each model with its vendor identity. This remains distinct from the backend
wire protocol—for example, Mistral or xAI may use an OpenAI-compatible API:

```yaml
routing:
  modelCards:
    - name: economical
      tags: [provider:mistral, tier:cheap]
    - name: reasoning
      tags: [provider:openai, tier:reasoning]

global:
  services:
    tenant_policy:
      enabled: true
      version: "2026-09-08"
      tenant_id_header: x-authz-tenant-id
      require_tenant: true
      currency: USD
      output_token_bound: 4096
      reasoning_token_bound: 8192
      require_pricing: true
      require_current_pricing: true
      default:
        allowed_providers: [mistral, openai]
        max_estimated_cost: 0.08
      tenants:
        economy:
          denied_models: [reasoning]
          max_estimated_cost: 0.01
```

Empty allowlists permit all configured values; deny lists win. A tenant entry
inherits default fields that it does not specify. Explicit empty lists clear an
inherited list. If `require_tenant` is true, requests without trusted identity
have no eligible model.

When `max_estimated_cost` is present, `currency` must be a three-letter uppercase
code, `output_token_bound` must be positive, and `require_pricing` must be true.
The tenant ceiling can only tighten a decision's `request_budget`; it cannot
raise it. This is a per-request guardrail, not a durable monthly quota.

## Product integration

Use the stable, non-generating endpoint to inspect a choice before execution:

```http
POST /v1/route/evaluate
Authorization: Bearer <management-token>
X-Authz-Tenant-Id: economy
Content-Type: application/json

{"model":"router/auto","text":"Explain DNS briefly."}
```

Python callers can use the schema-checking client:

```python
from vllm_sr import SemanticRouterClient

router = SemanticRouterClient("https://router.internal", token="...")
decision = router.evaluate(
    {"model": "router/auto", "text": "Explain DNS briefly."},
    tenant_id="economy",
)
print(decision.selection.selected_model)
```

The legacy `/api/v1/route/evaluate` endpoint remains available with its
`v1alpha1` response schema for compatibility. New products should require
`vllm-sr/routing-decision/v1` from the stable endpoint.
