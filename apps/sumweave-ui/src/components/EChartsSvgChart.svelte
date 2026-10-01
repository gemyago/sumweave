<script lang="ts">
  import { onMount } from 'svelte'
  import * as echarts from 'echarts/core'
  import { BarChart } from 'echarts/charts'
  import {
    AriaComponent,
    GridComponent,
    LegendComponent,
    TooltipComponent,
  } from 'echarts/components'
  import { SVGRenderer } from 'echarts/renderers'
  import type { ECharts, EChartsCoreOption } from 'echarts/core'

  interface Props {
    ariaLabel: string
    ariaDescription?: string
    option: EChartsCoreOption
    onDataClick?: (dataIndex: number) => void
    onLegendToggle?: (name: string) => void
    legendSelection?: Record<string, boolean>
    legendShortcuts?: Record<string, string>
    busy?: boolean
  }

  let { ariaLabel, ariaDescription, option, onDataClick, onLegendToggle, legendSelection, legendShortcuts, busy = false }: Props = $props()
  let chartElement: HTMLDivElement
  let chart: ECharts | undefined

  echarts.use([
    AriaComponent,
    BarChart,
    GridComponent,
    LegendComponent,
    SVGRenderer,
    TooltipComponent,
  ])

  function updateChart(nextOption: EChartsCoreOption) {
    chart?.setOption(nextOption, { notMerge: true })
  }

  function handleChartClick(params: unknown) {
    const dataIndex = (params as { dataIndex?: unknown }).dataIndex
    if (typeof dataIndex === 'number' && Number.isInteger(dataIndex) && dataIndex >= 0) {
      onDataClick?.(dataIndex)
    }
  }

  function handleLegendChange(params: unknown) {
    const name = (params as { name?: unknown }).name
    // ECharts toggles before emitting. Restore committed visibility synchronously,
    // even when the route ignores this intent while busy or the request fails.
    for (const [seriesName, selected] of Object.entries(legendSelection ?? {})) {
      chart?.dispatchAction({ type: selected ? 'legendSelect' : 'legendUnSelect', name: seriesName }, { silent: true })
    }
    if (typeof name === 'string' && Object.hasOwn(legendSelection ?? {}, name)) onLegendToggle?.(name)
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.altKey || event.ctrlKey || event.metaKey || event.repeat || event.isComposing) return
    const name = Object.entries(legendShortcuts ?? {}).find(([, key]) => key.toLowerCase() === event.key.toLowerCase())?.[0]
    if (!name) return
    event.preventDefault()
    onLegendToggle?.(name)
  }

  $effect(() => {
    updateChart(option)
  })

  onMount(() => {
    chart = echarts.init(chartElement, undefined, { renderer: 'svg' })
    chart.on('click', handleChartClick)
    chart.on('legendselectchanged', handleLegendChange)
    updateChart(option)

    const resizeObserver = typeof ResizeObserver === 'undefined'
      ? undefined
      : new ResizeObserver(() => chart?.resize())
    resizeObserver?.observe(chartElement)

    return () => {
      resizeObserver?.disconnect()
      chart?.off('click', handleChartClick)
      chart?.off('legendselectchanged', handleLegendChange)
      chart?.dispose()
      chart = undefined
    }
  })
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex (The SVG legend has no DOM keyboard targets; its named group supplies focused shortcuts.) -->
<div
  class="finance-chart-widget"
  role={onLegendToggle ? 'group' : 'img'}
  tabindex={onLegendToggle ? 0 : undefined}
  aria-label={ariaLabel}
  aria-describedby={ariaDescription}
  aria-keyshortcuts={legendShortcuts ? Object.values(legendShortcuts).join(' ') : undefined}
  aria-busy={onLegendToggle ? busy : undefined}
  onkeydown={handleKeydown}
  bind:this={chartElement}
></div>
