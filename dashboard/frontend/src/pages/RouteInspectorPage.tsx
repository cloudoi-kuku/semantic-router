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
