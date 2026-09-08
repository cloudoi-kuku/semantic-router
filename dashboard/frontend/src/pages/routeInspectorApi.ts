export const ROUTING_DECISION_SCHEMA_VERSION = 'vllm-sr/routing-decision/v1alpha1'

export interface RoutingDecisionRoute {
  recipe?: string
  decision?: string
  algorithm?: string
}

export interface RoutingDecisionSelection {
  status?: string
  method?: string
  selected_model?: string
  candidate_models?: string[]
  reason?: string
}

export interface RoutingDecisionEligibilityExclusion {
  model: string
  reasons: string[]
  missing_capabilities?: string[]
}

export interface RoutingDecisionEligibility {
  catalog_version: string
  required_capabilities?: string[]
  eligible_models?: string[]
  excluded_models?: RoutingDecisionEligibilityExclusion[]
}

export interface RoutingDecisionCost {
  catalog_version: string
  currency: string
  input_tokens: number
  output_tokens: number
  reasoning_tokens?: number
  max_cost: number
  candidates: Array<{
    model: string
    single_attempt_estimated_cost?: number
    estimated_cost?: number
    max_provider_attempts?: number
    eligible: boolean
    status: string
    price_version?: string
    price_source?: string
    effective_at?: string
    expires_at?: string
  }>
}

export interface RoutingDecisionResilience {
  contract_version: string
  type: string
  status: string
  max_attempts: number
  retry_on: string[]
  candidate_models?: string[]
  executes_models: boolean
}

export interface RoutingDecisionTenantPolicy {
  contract_version: string
  policy_version: string
  status: string
  source: string
  tenant_present: boolean
  require_tenant: boolean
  allowed_models?: string[]
  denied_models?: string[]
  allowed_providers?: string[]
  denied_providers?: string[]
  max_estimated_cost?: number
}

export interface RoutingDecisionWorkflow {
  contract_version: string
  type: string
  status: string
  authorization: { required_group: string; status: string }
  tool: {
    type: string
    provider?: string
    server_name?: string
    tool_name?: string
    max_results?: number
    timeout_seconds: number
    max_response_bytes: number
    max_evidence_characters?: number
    max_result_characters?: number
    requires_read_only?: boolean
  }
  synthesis_model?: string
  executes_tools: boolean
}

export interface RoutingDecisionSignals {
  used?: Record<string, unknown>
  matched?: Record<string, unknown>
  unmatched?: Record<string, unknown>
  confidences?: Record<string, number>
  values?: Record<string, number>
  metrics?: Record<string, unknown>
}

export interface RoutingDecisionTrace {
  decision_name?: string
  state?: string
  matched?: boolean
  confidence?: number
  root_trace?: unknown
}

export interface RoutingDecisionEnvelope {
  schema_version: string
  dry_run: boolean
  route: RoutingDecisionRoute
  selection: RoutingDecisionSelection
  eligibility?: RoutingDecisionEligibility
  cost?: RoutingDecisionCost
  workflow?: RoutingDecisionWorkflow
  resilience?: RoutingDecisionResilience
  tenant_policy?: RoutingDecisionTenantPolicy
  signals: RoutingDecisionSignals
  diagnostics?: {
    signal_errors?: Record<string, string>
    applied_unknown_policies?: Record<string, string>
    decision_error?: string
  }
  trace?: RoutingDecisionTrace[]
}

interface EvaluateRouteDecisionOptions {
  model: string
  text: string
  trace: boolean
  signal?: AbortSignal
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function assertRoutingDecisionEnvelope(value: unknown): asserts value is RoutingDecisionEnvelope {
  if (
    !isRecord(value) ||
    value.schema_version !== ROUTING_DECISION_SCHEMA_VERSION ||
    value.dry_run !== true ||
    !isRecord(value.route) ||
    !isRecord(value.selection) ||
    !isRecord(value.signals)
  ) {
    throw new Error('The Router returned an unsupported routing-decision response.')
  }
}

async function responseError(response: Response): Promise<Error> {
  try {
    const body = (await response.json()) as unknown
    if (isRecord(body)) {
      const message = body.message ?? body.error
      if (typeof message === 'string' && message.trim()) return new Error(message)
    }
  } catch {
    // Preserve a bounded status-only error when the upstream body is not JSON.
  }
  return new Error(`Routing inspection failed (${response.status}).`)
}

export async function evaluateRouteDecision({
  model,
  text,
  trace,
  signal,
}: EvaluateRouteDecisionOptions): Promise<RoutingDecisionEnvelope> {
  const endpoint = `/api/router/api/v1/route/evaluate${trace ? '?trace=true' : ''}`
  const response = await fetch(endpoint, {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ model: model.trim(), text: text.trim() }),
    signal,
  })
  if (!response.ok) throw await responseError(response)

  const body = (await response.json()) as unknown
  assertRoutingDecisionEnvelope(body)
  return body
}

export function populatedSignalEntries(
  signals: Record<string, unknown> | undefined,
): Array<[string, string]> {
  if (!signals) return []
  return Object.entries(signals).flatMap(([kind, value]) => {
    if (Array.isArray(value)) {
      const labels = value.filter((item): item is string => typeof item === 'string')
      return labels.length > 0 ? [[kind, labels.join(', ')]] : []
    }
    if (typeof value === 'string' && value.trim()) return [[kind, value]]
    return []
  })
}
