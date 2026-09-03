import { describe, expect, it } from 'vitest'

import {
  decisionRulesForSave,
  mergeDecisionForSave,
  type DecisionConfig,
} from './configPageSupport'

describe('decision editor preservation', () => {
  it('preserves prompt algorithms and non-form fields during edits', () => {
    const existing: DecisionConfig = {
      name: 'prompt-route',
      description: 'before',
      priority: 10,
      tier: 1,
      annotations: { owner: 'routing' },
      required_capabilities: ['chat', 'reasoning'],
      request_budget: {
        currency: 'USD',
        max_estimated_cost: 0.1,
        output_token_bound: 4096,
        require_pricing: true,
        require_current_pricing: true,
      },
      rules: {
        operator: 'AND',
        conditions: [{ type: 'keyword', name: 'complex' }],
      },
      modelRefs: [
        { model: 'model-a', use_reasoning: false },
        { model: 'model-b', use_reasoning: true },
      ],
      plugins: [],
      algorithm: {
        type: 'prompt',
        on_error: 'fallback',
        prompt: {
          model: 'router-small',
          instructions: 'Choose.',
          timeout_seconds: 5,
        },
      },
    }
    const updated = mergeDecisionForSave(existing, {
      name: existing.name,
      description: 'after',
      priority: 20,
      rules: existing.rules,
      modelRefs: existing.modelRefs,
      plugins: [],
    })

    expect(updated.description).toBe('after')
    expect(updated.algorithm).toEqual(existing.algorithm)
    expect(updated.annotations).toEqual(existing.annotations)
    expect(updated.required_capabilities).toEqual(['chat', 'reasoning'])
    expect(updated.request_budget).toEqual(existing.request_budget)
    expect(updated.tier).toBe(1)
  })

  it('preserves recursive rule trees during form edits', () => {
    const existing: DecisionConfig['rules'] = {
      operator: 'AND',
      on_unknown: 'fail_request',
      conditions: [
        { type: 'keyword', name: 'complex' },
        {
          operator: 'OR',
          conditions: [
            { type: 'metadata', name: 'canary' },
            {
              type: 'classifier',
              name: 'risk',
              label: 'RISKY',
              predicate: { gte: 0.8 },
              on_error: 'match',
            },
          ],
        },
      ],
    }

    const result = decisionRulesForSave(existing, {
      operator: 'AND',
      on_unknown: 'match',
      conditions: [{ type: 'keyword', name: 'flattened' }],
    })

    expect(result.operator).toBe(existing.operator)
    expect(result.conditions).toEqual(existing.conditions)
    expect(result.on_unknown).toBe('match')
    expect(result).not.toBe(existing)
  })
})
