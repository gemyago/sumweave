<script lang="ts">
  import { onMount } from 'svelte'
  import {
    CandlestickSeries,
    createChart,
    type CandlestickData,
    type IChartApi,
    type Time,
  } from 'lightweight-charts'
  import type { ChartCandleRow } from '../lib/data/charting'

  let { rows = [] } = $props<{ rows?: ChartCandleRow[] }>()

  let container = $state<HTMLDivElement | null>(null)
  let chart = $state<IChartApi | null>(null)
  let series = $state<{ setData(data: CandlestickData<Time>[]): void } | null>(null)

  onMount(() => {
    if (!container) {
      return
    }

    chart = createChart(container, {
      width: container.clientWidth || 720,
      height: 320,
      layout: {
        background: { color: 'transparent' },
        textColor: 'var(--text)',
      },
      grid: {
        vertLines: { color: 'transparent' },
        horzLines: { color: 'transparent' },
      },
      rightPriceScale: { borderVisible: false },
      timeScale: { borderVisible: false },
    })
    series = chart.addSeries(CandlestickSeries) as {
      setData(data: CandlestickData<Time>[]): void
    }

    return () => {
      chart?.remove()
      chart = null
      series = null
    }
  })

  $effect(() => {
    if (!series) {
      return
    }

    series.setData(
      rows.map((row: ChartCandleRow): CandlestickData<Time> => ({
        time: row.time as Time,
        open: row.open,
        high: row.high,
        low: row.low,
        close: row.close,
      })),
    )
  })
</script>

<div class="chart-shell">
  <div bind:this={container} class="chart" aria-label="Normalized candle chart"></div>
</div>

<style>
  .chart-shell {
    border: 1px solid var(--border);
    border-radius: var(--radius-default);
    background: var(--surface-raised);
    padding: var(--space-12);
  }

  .chart {
    width: 100%;
    min-height: 320px;
  }
</style>
