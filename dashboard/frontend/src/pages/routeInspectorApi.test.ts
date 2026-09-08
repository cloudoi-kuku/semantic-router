import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  evaluateRouteDecision,
  populatedSignalEntries,
  ROUTING_DECISION_SCHEMA_VERSION,
} from './routeInspectorApi'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('route inspector API', () => {
  it('requests a traceable dry-run decision through the dashboard proxy', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          schema_version: ROUTING_DECISION_SCHEMA_VERSION,
          dry_run: true,
          route: { decision: 'reasoning-route' },
          selection: { selected_model: 'niffy-reasoning' },
          eligibility: {
            catalog_version: 'vllm-sr/model-capability-catalog/v1alpha1',
            required_capabilities: ['chat', 'reasoning'],
            eligible_models: ['niffy-reasoning'],
          },
          workflow: {
            contract_version: 'vllm-sr/workflow/v1alpha1',
            type: 'web_search_answer',
            status: 'planned',
            authorization: { required_group: 'web-search-users', status: 'not_evaluated' },
            tool: {
              type: 'web_search',
              provider: 'searxng',
              max_results: 5,
              timeout_seconds: 5,
              max_response_bytes: 262144,
              max_evidence_characters: 8000,
            },
            synthesis_model: 'niffy-reasoning',
            executes_tools: false,
          },
          resilience: {
            contract_version: 'vllm-sr/resilience-plan/v1alpha1',
            type: 'ordered_fallback',
            status: 'planned',
            max_attempts: 2,
            retry_on: ['server_error'],
            candidate_models: ['niffy-reasoning', 'niffy-general'],
            executes_models: false,
          },
          signals: { matched: { keywords: ['reasoning_intent'] } },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    const result = await evaluateRouteDecision({
      model: ' niffy/auto ',
      text: ' Analyze this design. ',
      trace: true,
    })

    expect(result.selection.selected_model).toBe('niffy-reasoning')
    expect(result.eligibility?.required_capabilities).toEqual(['chat', 'reasoning'])
    expect(result.workflow?.executes_tools).toBe(false)
    expect(result.resilience?.executes_models).toBe(false)
    expect(result.resilience?.candidate_models).toEqual(['niffy-reasoning', 'niffy-general'])
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/router/api/v1/route/evaluate?trace=true',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ model: 'niffy/auto', text: 'Analyze this design.' }),
      }),
    )
  })

  it('rejects incompatible response contracts', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ schema_version: 'unknown', dry_run: true }), {
          status: 200,
        }),
      ),
    )

    await expect(
      evaluateRouteDecision({ model: 'niffy/auto', text: 'hello', trace: false }),
    ).rejects.toThrow(/unsupported routing-decision response/i)
  })

  it('accepts an inspect-only MCP workflow plan', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            schema_version: ROUTING_DECISION_SCHEMA_VERSION,
            dry_run: true,
            route: { decision: 'mcp-route' },
            selection: { selected_model: 'niffy-general' },
            workflow: {
              contract_version: 'vllm-sr/workflow/v1alpha1',
              type: 'mcp_tool_call',
              status: 'planned',
              authorization: { required_group: 'mcp-users', status: 'not_evaluated' },
              tool: {
                type: 'mcp',
                server_name: 'catalog',
                tool_name: 'lookup',
                timeout_seconds: 5,
                max_response_bytes: 262144,
                max_result_characters: 8000,
                requires_read_only: true,
              },
              executes_tools: false,
            },
            signals: {},
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      ),
    )
    const result = await evaluateRouteDecision({
      model: 'niffy/auto',
      text: 'lookup',
      trace: false,
    })
    expect(result.workflow?.tool).toMatchObject({
      type: 'mcp',
      tool_name: 'lookup',
      requires_read_only: true,
    })
    expect(result.workflow?.executes_tools).toBe(false)
  })

  it('only renders populated textual signal values', () => {
    expect(
      populatedSignalEntries({
        keywords: ['tool_intent'],
        classifiers: [],
        score: 0.91,
      }),
    ).toEqual([['keywords', 'tool_intent']])
  })
})
