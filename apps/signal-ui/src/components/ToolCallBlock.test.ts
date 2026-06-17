import { describe, it, expect } from 'vitest'
import { faker } from '@faker-js/faker'
import { render, screen } from '@testing-library/svelte'
import ToolCallBlock from './ToolCallBlock.svelte'

describe('ToolCallBlock', () => {
  it('renders a strategy quick link from nested strategy version responses', async () => {
    const strategyId = faker.string.uuid()
    const version = `v${faker.number.int({ min: 1, max: 9 })}`

    render(ToolCallBlock, {
      props: {
        name: 'sf_strategy_get_version',
        args: {},
        response: {
          version: {
            strategyId,
            version,
            displayName: 'Momentum breakout',
            status: 'ready',
            sourceType: 'saved',
            sourceLabel: 'AI draft',
            artifactHash: faker.string.hexadecimal({ length: 64, prefix: '' }),
            schemaVersion: 'strategy-dsl/v0',
            kind: 'momentum_cross',
            instrument: {
              venue: 'binance',
              symbol: 'BTCUSDT',
              assetClass: 'crypto_spot',
              active: true,
            },
            timeframe: '1h',
            parameterSummary: { fastWindow: 10, slowWindow: 20 },
            createdAt: '2026-01-01T00:00:00Z',
            updatedAt: '2026-01-01T00:00:00Z',
            definition: {
              kind: 'momentum_cross',
              instrument: {
                venue: 'binance',
                symbol: 'BTCUSDT',
                assetClass: 'crypto_spot',
                active: true,
              },
              timeframe: '1h',
              parameters: { fastWindow: 10, slowWindow: 20 },
            },
          },
        },
      },
    })

    expect(await screen.findByRole('link', { name: /Strategy /i })).toHaveAttribute(
      'href',
      `#/strategies/${encodeURIComponent(strategyId)}/${encodeURIComponent(version)}`,
    )
  })
})
