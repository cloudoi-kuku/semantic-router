# Ordered provider fallback

## Overview

`algorithm.type=fallback` tries `modelRefs` in declared order and returns the
first valid response. It advances only after a configured transport, timeout,
rate-limit, server, or invalid-response failure.

```yaml
modelRefs:
  - model: economical-model
  - model: alternate-provider-model
algorithm:
  type: fallback
  minimum_candidates: 2
  fallback:
    max_attempts: 2
    retry_on: [transport_error, timeout, rate_limited, server_error, invalid_response]
```

`max_attempts` is mandatory and cannot exceed the declared model count. A
decision `request_budget` bounds the cumulative worst-case chain. Per-model
cost estimates include that model's generated Envoy `retry_count`.

Provider models may also configure a router-side circuit:

```yaml
reliability:
  retry_count: 1
  circuit_breaker_failures: 3
  circuit_breaker_open_time: 30s
```

The circuit observes 408, 429, and 5xx outcomes. Once open, the model is
excluded from new decisions until the interval expires; a successful response
resets its failure history. Dry-run reports the bounded chain but performs no
provider calls or health probes.

## Key Advantages

- Real cross-provider failover through the Router's normalized provider path.
- Explicit attempt and failure-class bounds.
- Conservative retry-aware cost admission and health-open filtering.

## What Problem Does It Solve?

It keeps availability failures from pinning a product to one provider without
turning every successful economical response into a premium-model call.

## When to Use

Use it for availability fallback across compatible model capabilities. Use the
confidence algorithm when a validated response-quality signal—not provider
failure—should trigger escalation.

## Configuration

The examples above show both decision and provider configuration.

## Dependencies and Limitations

- Requires a reachable `global.integrations.looper.endpoint`.
- Every fallback model must satisfy the decision's capability and policy gates.
- Circuits are process-local and reset when the Router restarts.
- Dry-run inspects current circuit state but does not actively probe providers.
