import React, { useState } from 'react'

import ProductIcon from '../components/ProductIcon'
import {
  evaluateRouteDecision,
  populatedSignalEntries,
  type RoutingDecisionEnvelope,
} from './routeInspectorApi'
import styles from './RouteInspectorPage.module.css'

const DEFAULT_MODEL = 'niffy/auto'

const RouteInspectorPage: React.FC = () => {
  const [model, setModel] = useState(DEFAULT_MODEL)
  const [prompt, setPrompt] = useState('')
  const [includeTrace, setIncludeTrace] = useState(true)
  const [result, setResult] = useState<RoutingDecisionEnvelope | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [isEvaluating, setIsEvaluating] = useState(false)

  const matchedSignals = populatedSignalEntries(result?.signals.matched)
  const usedSignals = populatedSignalEntries(result?.signals.used)
  const canEvaluate = prompt.trim().length > 0 && model.trim().length > 0 && !isEvaluating

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!canEvaluate) return

    setIsEvaluating(true)
    setError(null)
    try {
      setResult(await evaluateRouteDecision({ model, text: prompt, trace: includeTrace }))
    } catch (cause) {
      setResult(null)
      setError(cause instanceof Error ? cause.message : 'Routing inspection failed.')
    } finally {
      setIsEvaluating(false)
    }
  }

  return (
    <main className={styles.container} data-testid="route-inspector-page">
      <header className={styles.masthead}>
        <div>
          <span className={styles.eyebrow}>Staging decision</span>
          <h1>Route Inspector</h1>
          <p>Preview the live routing choice without invoking a model or tool.</p>
        </div>
        <span className={styles.noCostBadge}>
          <ProductIcon name="eye" />
          No provider generation
        </span>
      </header>

      <section className={styles.workspace}>
        <form className={styles.formPanel} onSubmit={(event) => void handleSubmit(event)}>
          <div className={styles.panelHeading}>
            <div>
              <span>Input</span>
              <h2>Stage a request</h2>
            </div>
            <span className={styles.privacyLabel}>Prompt stays out of the response</span>
          </div>

          <label className={styles.field}>
            <span>Routing model</span>
            <input
              value={model}
              onChange={(event) => setModel(event.target.value)}
              autoComplete="off"
              spellCheck={false}
            />
          </label>

          <label className={styles.field}>
            <span>Prompt</span>
            <textarea
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder="Describe the task whose route you want to inspect."
              rows={8}
            />
          </label>

          <label className={styles.traceToggle}>
            <input
              type="checkbox"
              checked={includeTrace}
              onChange={(event) => setIncludeTrace(event.target.checked)}
            />
            Include privacy-safe decision trace
          </label>

          <button className={styles.evaluateButton} type="submit" disabled={!canEvaluate}>
            <ProductIcon name="search" />
            {isEvaluating ? 'Evaluating…' : 'Inspect route'}
          </button>
        </form>

        <section className={styles.resultPanel} aria-live="polite">
          <div className={styles.panelHeading}>
            <div>
              <span>Result</span>
              <h2>Routing decision</h2>
            </div>
            {result ? <span className={styles.schemaBadge}>{result.schema_version}</span> : null}
          </div>

          {error ? (
            <div className={styles.error} role="alert">
              {error}
            </div>
          ) : null}
          {!result && !error ? (
            <div className={styles.emptyState}>
              <ProductIcon name="decision" />
              <strong>No staged decision yet</strong>
              <span>Submit a prompt to inspect the live policy path.</span>
            </div>
          ) : null}

          {result ? (
            <div className={styles.resultBody}>
              <dl className={styles.decisionGrid}>
                <div>
                  <dt>Decision</dt>
                  <dd>{result.route.decision || 'Unresolved'}</dd>
                </div>
                <div>
                  <dt>Selected model</dt>
                  <dd>{result.selection.selected_model || 'None'}</dd>
                </div>
                <div>
                  <dt>Recipe</dt>
                  <dd>{result.route.recipe || 'Default'}</dd>
                </div>
                <div>
                  <dt>Algorithm</dt>
                  <dd>{result.route.algorithm || 'Not reported'}</dd>
                </div>
                <div>
                  <dt>Selection method</dt>
                  <dd>{result.selection.method || 'Not reported'}</dd>
                </div>
                <div>
                  <dt>Status</dt>
                  <dd>{result.selection.status || 'Unknown'}</dd>
                </div>
              </dl>

              {result.selection.reason ? (
                <div className={styles.explanation}>
                  <span>Why this model</span>
                  <p>{result.selection.reason}</p>
                </div>
              ) : null}

              {result.tenant_policy ? (
                <div className={styles.signalSection}>
                  <h3>Tenant policy</h3>
                  <p className={styles.muted}>
                    {result.tenant_policy.contract_version} · policy{' '}
                    {result.tenant_policy.policy_version || 'unversioned'}
                  </p>
                  <p>
                    Status: {result.tenant_policy.status}; source: {result.tenant_policy.source};
                    trusted tenant identity:{' '}
                    {result.tenant_policy.tenant_present ? 'present' : 'absent'}.
                  </p>
                  <p className={styles.muted}>
                    Allowed providers:{' '}
                    {(result.tenant_policy.allowed_providers || []).join(', ') || 'all'}; maximum
                    estimated cost:{' '}
                    {result.tenant_policy.max_estimated_cost?.toFixed(6) ?? 'decision default'}.
                  </p>
                </div>
              ) : null}

              {result.eligibility ? (
                <div className={styles.signalSection}>
                  <h3>Model eligibility</h3>
                  <p className={styles.muted}>{result.eligibility.catalog_version}</p>
                  <div className={styles.signalList}>
                    {(result.eligibility.required_capabilities || []).map((capability) => (
                      <span className={styles.usedSignal} key={`required-${capability}`}>
                        requires · {capability}
                      </span>
                    ))}
                    {(result.eligibility.eligible_models || []).map((candidate) => (
                      <span className={styles.matchedSignal} key={`eligible-${candidate}`}>
                        eligible · {candidate}
                      </span>
                    ))}
                  </div>
                  {(result.eligibility.excluded_models || []).map((excluded) => (
                    <p className={styles.muted} key={`excluded-${excluded.model}`}>
                      Excluded {excluded.model}: {excluded.reasons.join(', ')}
                      {excluded.missing_capabilities?.length
                        ? ` (${excluded.missing_capabilities.join(', ')})`
                        : ''}
                    </p>
                  ))}
                </div>
              ) : null}

              {result.cost ? (
                <div className={styles.signalSection}>
                  <h3>Estimated request cost</h3>
                  <p className={styles.muted}>{result.cost.catalog_version}</p>
                  <p>
                    Bound: {result.cost.input_tokens} input + {result.cost.output_tokens} output
                    {result.cost.reasoning_tokens
                      ? ` + ${result.cost.reasoning_tokens} reasoning`
                      : ''}{' '}
                    tokens; ceiling {result.cost.max_cost.toFixed(6)} {result.cost.currency}
                  </p>
                  {(result.cost.candidates || []).map((candidate) => (
                    <p className={styles.muted} key={`cost-${candidate.model}`}>
                      {candidate.model}: {candidate.estimated_cost?.toFixed(6) ?? 'unpriced'}{' '}
                      {result.cost?.currency} ({candidate.status}; up to{' '}
                      {candidate.max_provider_attempts || 1} provider attempt
                      {(candidate.max_provider_attempts || 1) === 1 ? '' : 's'})
                    </p>
                  ))}
                </div>
              ) : null}

              {result.resilience ? (
                <div className={styles.signalSection}>
                  <h3>Planned resilience</h3>
                  <p className={styles.muted}>{result.resilience.contract_version}</p>
                  <p>
                    Ordered chain:{' '}
                    {(result.resilience.candidate_models || []).join(' → ') || 'none'}
                  </p>
                  <p className={styles.muted}>
                    At most {result.resilience.max_attempts} model attempts; advances on{' '}
                    {result.resilience.retry_on.join(', ')}. Dry-run model execution:{' '}
                    {result.resilience.executes_models ? 'yes' : 'no'}.
                  </p>
                </div>
              ) : null}

              {result.workflow ? (
                <div className={styles.signalSection}>
                  <h3>Planned workflow</h3>
                  <p className={styles.muted}>{result.workflow.contract_version}</p>
                  <p>
                    {result.workflow.type} via{' '}
                    {result.workflow.tool.type === 'mcp'
                      ? `${result.workflow.tool.server_name}.${result.workflow.tool.tool_name} (read-only required)`
                      : result.workflow.tool.provider}
                    ; synthesis model {result.workflow.synthesis_model || 'not selected'}
                  </p>
                  <p className={styles.muted}>
                    Authorization: {result.workflow.authorization.status}; required group{' '}
                    {result.workflow.authorization.required_group}. Dry-run tool execution:{' '}
                    {result.workflow.executes_tools ? 'yes' : 'no'}.
                  </p>
                  <p className={styles.muted}>
                    Bounds:{' '}
                    {result.workflow.tool.type === 'web_search'
                      ? `${result.workflow.tool.max_results} results, `
                      : ''}
                    {result.workflow.tool.timeout_seconds}s timeout,{' '}
                    {result.workflow.tool.max_response_bytes} response bytes,{' '}
                    {result.workflow.tool.type === 'mcp'
                      ? result.workflow.tool.max_result_characters
                      : result.workflow.tool.max_evidence_characters}{' '}
                    injected characters.
                  </p>
                </div>
              ) : null}

              <div className={styles.signalSection}>
                <h3>Signals</h3>
                {[...usedSignals, ...matchedSignals].length > 0 ? (
                  <div className={styles.signalList}>
                    {usedSignals.map(([kind, value]) => (
                      <span className={styles.usedSignal} key={`used-${kind}-${value}`}>
                        used · {kind}: {value}
                      </span>
                    ))}
                    {matchedSignals.map(([kind, value]) => (
                      <span className={styles.matchedSignal} key={`matched-${kind}-${value}`}>
                        matched · {kind}: {value}
                      </span>
                    ))}
                  </div>
                ) : (
                  <p className={styles.muted}>No named signals were reported.</p>
                )}
              </div>

              {result.trace?.length ? (
                <details className={styles.tracePanel}>
                  <summary>Decision trace ({result.trace.length})</summary>
                  <div className={styles.traceList}>
                    {result.trace.map((entry, index) => (
                      <div key={`${entry.decision_name || 'decision'}-${index}`}>
                        <strong>{entry.decision_name || `Decision ${index + 1}`}</strong>
                        <span>{entry.state || (entry.matched ? 'matched' : 'not matched')}</span>
                        {typeof entry.confidence === 'number' ? (
                          <small>confidence {entry.confidence.toFixed(2)}</small>
                        ) : null}
                      </div>
                    ))}
                  </div>
                </details>
              ) : null}
            </div>
          ) : null}
        </section>
      </section>
    </main>
  )
}

export default RouteInspectorPage
