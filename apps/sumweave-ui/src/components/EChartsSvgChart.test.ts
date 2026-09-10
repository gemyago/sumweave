import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import EChartsSvgChart from './EChartsSvgChart.svelte'

const mocks = vi.hoisted(() => ({
  dispose: vi.fn(),
  init: vi.fn(),
  resize: vi.fn(),
  setOption: vi.fn(),
  use: vi.fn(),
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
    mocks.init.mockReset().mockReturnValue({
      dispose: mocks.dispose,
      resize: mocks.resize,
      setOption: mocks.setOption,
    })
    mocks.resize.mockReset()
    mocks.setOption.mockReset()
    mocks.use.mockReset()
    ResizeObserverMock.instances = []
    vi.stubGlobal('ResizeObserver', ResizeObserverMock)
  })

  it('initializes an SVG chart with an accessible name and updates options', async () => {
    const firstOption = { series: [] }
    const secondOption = { series: [{ type: 'bar', data: [42] }] }
    const view = render(EChartsSvgChart, { ariaLabel: 'Cash flow chart', option: firstOption })

    expect(await screen.findByRole('img', { name: 'Cash flow chart' })).toBeInTheDocument()
    expect(mocks.use).toHaveBeenCalledOnce()
    expect(mocks.init).toHaveBeenCalledWith(expect.any(HTMLDivElement), undefined, { renderer: 'svg' })
    expect(mocks.setOption).toHaveBeenLastCalledWith(firstOption, { notMerge: true })

    await view.rerender({ ariaLabel: 'Cash flow chart', option: secondOption })

    expect(mocks.setOption).toHaveBeenLastCalledWith(secondOption, { notMerge: true })
  })

  it('resizes on container changes and cleans up its observer and chart', async () => {
    const view = render(EChartsSvgChart, { ariaLabel: 'Cash flow chart', option: { series: [] } })
    await screen.findByRole('img', { name: 'Cash flow chart' })

    ResizeObserverMock.instances[0].trigger()
    expect(mocks.resize).toHaveBeenCalledOnce()

    view.unmount()
    expect(ResizeObserverMock.instances[0].disconnect).toHaveBeenCalledOnce()
    expect(mocks.dispose).toHaveBeenCalledOnce()
  })
})
