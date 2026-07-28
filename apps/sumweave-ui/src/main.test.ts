import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  appComponent: Symbol('App'),
  mount: vi.fn((component: unknown, options: { target: Element | Document | ShadowRoot }) => {
    const mounted = document.createElement('div')
    mounted.setAttribute('data-testid', 'mock-app-shell')
    options.target.appendChild(mounted)
    return { component }
  }),
}))

vi.mock('svelte', async (importOriginal) => {
  const actual = await importOriginal<typeof import('svelte')>()
  return {
    ...actual,
    mount: mocks.mount,
  }
})

vi.mock('./App.svelte', () => ({
  default: mocks.appComponent,
}))

describe('main entry', () => {
  beforeEach(() => {
    document.body.innerHTML = '<div id="app"></div>'
    mocks.mount.mockClear()
  })

  afterEach(() => {
    vi.resetModules()
  })

  it('mounts the app shell into #app', async () => {
    await import('./main')
    const root = document.getElementById('app')
    expect(mocks.mount).toHaveBeenCalledWith(mocks.appComponent, expect.objectContaining({ target: root }))
    expect(root?.childElementCount).toBeGreaterThan(0)
  })
})
