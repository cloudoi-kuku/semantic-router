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
