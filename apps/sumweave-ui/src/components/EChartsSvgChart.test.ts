import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import EChartsSvgChart from './EChartsSvgChart.svelte'

const mocks = vi.hoisted(() => ({
  dispose: vi.fn(),
  dispatchAction: vi.fn(),
  off: vi.fn(),
  on: vi.fn(),
  init: vi.fn(),
  resize: vi.fn(),
  setOption: vi.fn(),
  use: vi.fn(),
  clickHandler: undefined as ((params: unknown) => void) | undefined,
  legendHandler: undefined as ((params: unknown) => void) | undefined,
}))

vi.mock('echarts/core', () => ({
  init: mocks.init,
  use: mocks.use,
}))

vi.mock('echarts/charts', () => ({ BarChart: class BarChart {} }))
vi.mock('echarts/components', () => ({
  AriaComponent: class AriaComponent {},
  GridComponent: class GridComponent {},
  LegendComponent: class LegendComponent {},
  TooltipComponent: class TooltipComponent {},
}))
vi.mock('echarts/renderers', () => ({ SVGRenderer: class SVGRenderer {} }))

class ResizeObserverMock {
  static instances: ResizeObserverMock[] = []
  readonly observe = vi.fn()
  readonly disconnect = vi.fn()
  private readonly callback: ResizeObserverCallback

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback
    ResizeObserverMock.instances.push(this)
  }

  trigger() {
    this.callback([], this as unknown as ResizeObserver)
  }
}

describe('EChartsSvgChart', () => {
  beforeEach(() => {
    mocks.dispose.mockReset()
    mocks.dispatchAction.mockReset()
    mocks.off.mockReset()
    mocks.on.mockReset().mockImplementation((eventName: string, handler: (params: unknown) => void) => {
      if (eventName === 'click') mocks.clickHandler = handler
      if (eventName === 'legendselectchanged') mocks.legendHandler = handler
    })
    mocks.init.mockReset().mockReturnValue({
      dispose: mocks.dispose,
      dispatchAction: mocks.dispatchAction,
      off: mocks.off,
      on: mocks.on,
      resize: mocks.resize,
      setOption: mocks.setOption,
    })
    mocks.resize.mockReset()
    mocks.setOption.mockReset()
    mocks.use.mockReset()
    mocks.clickHandler = undefined
    mocks.legendHandler = undefined
    ResizeObserverMock.instances = []
    vi.stubGlobal('ResizeObserver', ResizeObserverMock)
  })

  it('initializes an SVG chart with an accessible name, description, and updated options', async () => {
    const firstOption = { series: [] }
    const secondOption = { series: [{ type: 'bar', data: [42] }] }
    const view = render(EChartsSvgChart, {
      ariaLabel: 'Cash flow chart',
      ariaDescription: 'cash-flow-chart-description',
      option: firstOption,
    })

    expect(await screen.findByRole('img', { name: 'Cash flow chart' })).toHaveAttribute(
      'aria-describedby',
      'cash-flow-chart-description',
    )
    expect(mocks.use).toHaveBeenCalledOnce()
    expect(mocks.init).toHaveBeenCalledWith(expect.any(HTMLDivElement), undefined, { renderer: 'svg' })
    expect(mocks.setOption).toHaveBeenLastCalledWith(firstOption, { notMerge: true })

    await view.rerender({
      ariaLabel: 'Cash flow chart',
      ariaDescription: 'cash-flow-chart-description',
      option: secondOption,
    })

    expect(mocks.setOption).toHaveBeenLastCalledWith(secondOption, { notMerge: true })
  })

  it('resizes on container changes and cleans up its observer and chart', async () => {
    const view = render(EChartsSvgChart, { ariaLabel: 'Cash flow chart', option: { series: [] } })
    await screen.findByRole('img', { name: 'Cash flow chart' })

    ResizeObserverMock.instances[0].trigger()
    expect(mocks.resize).toHaveBeenCalledOnce()

    view.unmount()
    expect(ResizeObserverMock.instances[0].disconnect).toHaveBeenCalledOnce()
    expect(mocks.off).toHaveBeenCalledWith('click', expect.any(Function))
    expect(mocks.off).toHaveBeenCalledWith('legendselectchanged', expect.any(Function))
    expect(mocks.dispose).toHaveBeenCalledOnce()
  })

  it('forwards valid clicks on chart data items', async () => {
    const onDataClick = vi.fn()
    render(EChartsSvgChart, { ariaLabel: 'Cash flow chart', option: { series: [] }, onDataClick })
    await screen.findByRole('img', { name: 'Cash flow chart' })

    mocks.clickHandler?.({ dataIndex: 2 })
    mocks.clickHandler?.({ dataIndex: '2' })

    expect(onDataClick).toHaveBeenCalledOnce()
    expect(onDataClick).toHaveBeenCalledWith(2)
  })

  it('restores committed legend selection silently before forwarding native intent', async () => {
    const onLegendToggle = vi.fn(() => {
      expect(mocks.dispatchAction).toHaveBeenCalledWith({ type: 'legendSelect', name: 'Income' }, { silent: true })
      expect(mocks.dispatchAction).toHaveBeenCalledWith({ type: 'legendUnSelect', name: 'Expense' }, { silent: true })
    })
    render(EChartsSvgChart, { ariaLabel: 'Cash flow chart', option: {}, legendSelection: { Income: true, Expense: false }, onLegendToggle })
    await screen.findByRole('group', { name: 'Cash flow chart' })
    mocks.legendHandler?.({ name: 'Income', selected: { Income: false, Expense: false } })
    expect(onLegendToggle).toHaveBeenCalledExactlyOnceWith('Income')
    mocks.legendHandler?.({ name: 'Unknown' })
    expect(onLegendToggle).toHaveBeenCalledOnce()
  })

  it('uses only unmodified, non-repeating focused-chart shortcuts and preserves focus through option updates', async () => {
    const onLegendToggle = vi.fn()
    const user = userEvent.setup()
    const props = { ariaLabel: 'Cash flow chart', option: {}, onLegendToggle, legendShortcuts: { Income: 'I', Expense: 'E' } }
    const view = render(EChartsSvgChart, props)
    const chart = await screen.findByRole('group', { name: 'Cash flow chart' })
    await user.keyboard('i')
    expect(onLegendToggle).not.toHaveBeenCalled()
    chart.focus()
    await user.keyboard('{Control>}i{/Control}x{Enter} ')
    chart.dispatchEvent(new KeyboardEvent('keydown', { key: 'i', repeat: true }))
    chart.dispatchEvent(new KeyboardEvent('keydown', { key: 'i', isComposing: true }))
    expect(onLegendToggle).not.toHaveBeenCalled()
    await user.keyboard('iE')
    expect(onLegendToggle.mock.calls).toEqual([['Income'], ['Expense']])
    await view.rerender({ ...props, option: { series: [] } })
    expect(chart).toHaveFocus()
  })
})
