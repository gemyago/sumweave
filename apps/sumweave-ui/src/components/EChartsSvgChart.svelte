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
  }

  let { ariaLabel, ariaDescription, option, onDataClick }: Props = $props()
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

  $effect(() => {
    updateChart(option)
  })

  onMount(() => {
    chart = echarts.init(chartElement, undefined, { renderer: 'svg' })
    chart.on('click', handleChartClick)
    updateChart(option)

    const resizeObserver = typeof ResizeObserver === 'undefined'
      ? undefined
      : new ResizeObserver(() => chart?.resize())
    resizeObserver?.observe(chartElement)

    return () => {
      resizeObserver?.disconnect()
      chart?.off('click', handleChartClick)
      chart?.dispose()
      chart = undefined
    }
  })
</script>

<div
  class="finance-chart-widget"
  role="img"
  aria-label={ariaLabel}
  aria-describedby={ariaDescription}
  bind:this={chartElement}
></div>
