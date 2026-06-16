import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import UtcDateRangePicker from './UtcDateRangePicker.svelte'

describe('UtcDateRangePicker', () => {
  it('renders explicit utc values and resolves presets once', async () => {
    render(UtcDateRangePicker, {
      props: {
        startValue: '2026-06-15T00:00:00.000Z',
        endValue: '2026-06-16T00:00:00.000Z',
        presetAnchorIso: '2026-06-16T12:00:00.000Z',
      },
    })
    const user = userEvent.setup()

    expect(screen.getByText('2026-06-15T00:00:00.000Z')).toBeInTheDocument()
    expect(screen.getByText('2026-06-16T00:00:00.000Z')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Last 24h' }))

    expect(screen.getByText('2026-06-15T12:00:00.000Z')).toBeInTheDocument()
    expect(screen.getAllByText('2026-06-16T12:00:00.000Z').length).toBeGreaterThan(0)
    expect(screen.getByText('Preset resolved once from 2026-06-16T12:00:00.000Z.')).toBeInTheDocument()
  })

  it('shows inline validation for bounded and reversed ranges when requested', () => {
    render(UtcDateRangePicker, {
      props: {
        startValue: '2026-06-15T13:00:00.000Z',
        endValue: '2026-06-15T12:00:00.000Z',
        minIso: '2026-06-15T11:30:00.000Z',
        maxIso: '2026-06-15T12:30:00.000Z',
        outOfBoundsMessage: 'UTC range must stay within the selected availability window.',
        showValidation: true,
      },
    })

    expect(screen.getByRole('alert')).toHaveTextContent('UTC start must be earlier than UTC end.')
    expect(screen.getByRole('alert')).toHaveTextContent(
      'UTC range must stay within the selected availability window.',
    )
  })
})
