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
