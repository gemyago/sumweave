import { describe, expect, it } from 'vitest'
import { faker } from '@faker-js/faker'
import { toChartCandleRows } from './charting'
import type { DataCandle } from './data-api'

describe('toChartCandleRows', () => {
  it('maps chart values while retaining provenance-bearing candle rows', () => {
    const start = faker.date.recent()
    const candle: DataCandle = {
      identity: faker.number.int({ min: 1, max: 1000 }),
      venue: 'hyperliquid-perps',
      symbol: 'BTCUSD',
      assetClass: 'crypto',
      timeframe: '1m',
      start,
      end: faker.date.soon({ refDate: start }),
      open: 100,
      high: 110,
      low: 95,
      close: 108,
      volume: 7,
      quality: 'validated',
      provenanceSource: faker.word.noun(),
      provenanceIdentity: faker.string.uuid(),
    }

    const [row] = toChartCandleRows([candle])

    expect(row).toEqual(
      expect.objectContaining({
        time: Math.floor(start.getTime() / 1000),
        open: candle.open,
        high: candle.high,
        low: candle.low,
        close: candle.close,
      }),
    )
    expect(row.candle.provenanceSource).toBe(candle.provenanceSource)
    expect(row.candle.provenanceIdentity).toBe(candle.provenanceIdentity)
  })
})
