import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import { tick } from 'svelte'
import { faker } from '@faker-js/faker'
import * as echarts from 'echarts/core'
import Finance from './Finance.svelte'
import { FinanceShellState } from '../lib/finance/shell-state.svelte'

const mocks = vi.hoisted(() => ({ api: {} as Record<string, ReturnType<typeof vi.fn>>, shell: null as FinanceShellState | null }))

vi.mock('../lib/finance/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/api')>()),
  createSignalFinanceApiForAuth: () => mocks.api,
}))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'local-test-token' } }))
vi.mock('../lib/finance/shell-state.svelte', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/shell-state.svelte')>()),
  useFinanceShellState: () => mocks.shell!,
}))

async function fixture() {
  window.localStorage.clear()
  window.location.hash = '#/finance'
  mocks.shell = new FinanceShellState()
  const startDate = faker.date.recent()
  const endDate = new Date(startDate.getTime() + 86400000)
  const tenantId = faker.string.uuid()
  const period = { startDate, endDate }
  const totals = { displayCurrency: 'USD', incomeMinor: 10000, expenseMinor: 5000, netMinor: 5000, transactionCount: 2, complete: true }
  mocks.api = {
    listTenants: vi.fn().mockResolvedValue([{ id: tenantId, name: faker.company.name(), displayCurrency: 'USD', joinedAt: startDate, createdAt: startDate, updatedAt: startDate }]),
    getDashboard: vi.fn().mockResolvedValue({ period, settled: totals, pending: totals, categoryBreakdowns: [], accountBalances: [], alerts: [], fxCoverage: [], nativeSettledTotals: [] }),
    getCashFlowSeries: vi.fn().mockResolvedValue({ period, groupBy: 'day', displayCurrency: 'USD', complete: true, missingFx: [], buckets: [{ ...period, incomeMinor: 10000, expenseMinor: 5000 }] }),
    listAccounts: vi.fn().mockResolvedValue([]),
    listTransactions: vi.fn().mockResolvedValue([]),
    listConnections: vi.fn().mockResolvedValue([]),
  }
  // jsdom has no layout/canvas. Only dimensions/text metrics are supplied;
  // ECharts, zrender, SVG rendering, actions and the dashboard are all real.
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(308)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(300)
  echarts.setPlatformAPI({ measureText: (text) => ({ width: text.length * 7 }) })
  const view = render(Finance)
  const host = await screen.findByRole('group', { name: 'Cash flow chart' })
  const chart = echarts.getInstanceByDom(host)!
  await waitFor(() => expect(chart.getOption().series).toHaveLength(2))
  return { view, host, chart }
}

function geometry(chart: echarts.ECharts, host: HTMLElement) {
  chart.getZr().flush()
  // Bar rectangles include removed/fading elements still in the display list,
  // unlike series data or legend.selected, which miss the original regression.
  const bars = chart.getZr().storage.getDisplayList(true)
    .filter((element) => element.type === 'rect' && !element.ignore && !element.invisible &&
      ['var(--color-success)', 'var(--color-danger)'].includes(element.style.fill))
    .map((element) => ({ fill: element.style.fill, bounds: { ...element.getBoundingRect() } }))
  const svgBars = Array.from(host.querySelectorAll('path'))
    .filter((path) => ['var(--color-success)', 'var(--color-danger)'].includes(path.getAttribute('fill') ?? ''))
    .map((path) => path.getAttribute('d'))
  return { bars, svgBars }
}

describe('real ECharts cash-flow rendering', () => {
  it.each(['both', 'income', 'none'] as const)('restores %s geometry immediately and ignores busy toggles until atomic success', async (inclusion) => {
    const { view, host, chart } = await fixture()
    try {
      if (inclusion !== 'both') {
        chart.dispatchAction({ type: 'legendToggleSelect', name: 'Expense' })
        await waitFor(() => expect(chart.getOption().legend).toMatchObject([{ selected: { Expense: false } }]))
      }
      if (inclusion === 'none') {
        chart.dispatchAction({ type: 'legendToggleSelect', name: 'Income' })
        await waitFor(() => expect(chart.getOption().legend).toMatchObject([{ selected: { Income: false } }]))
      }
      // Settle initial/committed animations before taking the baseline. This also
      // makes the test detect ghost geometry if animation:false is removed.
      await new Promise((resolve) => setTimeout(resolve, 1100))
      const baseline = geometry(chart, host)
      expect(baseline.bars).toHaveLength(inclusion === 'both' ? 2 : inclusion === 'income' ? 1 : 0)
      const href = screen.getByRole('link', { name: 'View all transactions' }).getAttribute('href')
      let release!: (value: unknown[]) => void
      mocks.api.listTransactions.mockReturnValueOnce(new Promise((resolve) => { release = resolve }))
      const before = mocks.api.listTransactions.mock.calls.length
      chart.dispatchAction({ type: 'legendToggleSelect', name: 'Expense' })
      expect(geometry(chart, host)).toEqual(baseline)
      await tick()
      expect(host).toHaveAttribute('aria-busy', 'true')
      chart.dispatchAction({ type: 'legendToggleSelect', name: 'Income' })
      expect(geometry(chart, host)).toEqual(baseline)
      chart.dispatchAction({ type: 'legendToggleSelect', name: 'Expense' })
      expect(geometry(chart, host)).toEqual(baseline)
      for (let frame = 0; frame < 12; frame++) {
        await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
        expect(geometry(chart, host)).toEqual(baseline)
      }
      expect(mocks.api.listTransactions.mock.calls.length).toBe(before + 1)
      expect(screen.getByRole('link', { name: 'View all transactions' })).toHaveAttribute('href', href)
      release([])
      await waitFor(() => expect(host).toHaveAttribute('aria-busy', 'false'))
      expect(geometry(chart, host).bars).toHaveLength(inclusion === 'both' ? 1 : inclusion === 'income' ? 2 : 1)
      expect(screen.getByRole('link', { name: 'View all transactions' }).getAttribute('href')).not.toBe(href)
    } finally {
      view.unmount()
      vi.restoreAllMocks()
    }
  })

  it('keeps zero bar geometry after a rejected inclusion request and permits retry', async () => {
    const { view, host, chart } = await fixture()
    try {
      chart.dispatchAction({ type: 'legendToggleSelect', name: 'Income' })
      await waitFor(() => expect(chart.getOption().legend).toMatchObject([{ selected: { Income: false } }]))
      chart.dispatchAction({ type: 'legendToggleSelect', name: 'Expense' })
      await waitFor(() => expect(chart.getOption().legend).toMatchObject([{ selected: { Expense: false } }]))
      await new Promise((resolve) => setTimeout(resolve, 1100))
      const baseline = geometry(chart, host)
      expect(baseline.bars).toHaveLength(0)
      let reject!: (error: Error) => void
      mocks.api.listTransactions.mockReturnValueOnce(new Promise((_, fail) => { reject = fail }))
      chart.dispatchAction({ type: 'legendToggleSelect', name: 'Expense' })
      expect(geometry(chart, host)).toEqual(baseline)
      await tick()
      reject(new Error('Local rendering regression failure'))
      await waitFor(() => expect(host).toHaveAttribute('aria-busy', 'false'))
      expect(geometry(chart, host)).toEqual(baseline)
      chart.dispatchAction({ type: 'legendToggleSelect', name: 'Expense' })
      await waitFor(() => expect(chart.getOption().legend).toMatchObject([{ selected: { Expense: true } }]))
      expect(geometry(chart, host).bars).toHaveLength(1)
    } finally {
      view.unmount()
      vi.restoreAllMocks()
    }
  })
})
