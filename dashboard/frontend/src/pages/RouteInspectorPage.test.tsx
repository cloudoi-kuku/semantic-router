import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'

import RouteInspectorPage from './RouteInspectorPage'

describe('RouteInspectorPage', () => {
  it('presents inspection as a non-generating staging action', () => {
    const markup = renderToStaticMarkup(<RouteInspectorPage />)

    expect(markup).toContain('Route Inspector')
    expect(markup).toContain('No provider generation')
    expect(markup).toContain('Prompt stays out of the response')
    expect(markup).toContain('Inspect route')
  })
})
