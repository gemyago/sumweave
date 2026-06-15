import type { DataCandle } from './data-api'

export interface ChartCandleRow {
  time: number
  open: number
  high: number
  low: number
  close: number
  candle: DataCandle
}

export function toChartCandleRows(candles: DataCandle[]): ChartCandleRow[] {
  return candles.map((candle) => ({
    time: Math.floor(candle.start.getTime() / 1000),
    open: candle.open,
    high: candle.high,
    low: candle.low,
    close: candle.close,
    candle,
  }))
}
