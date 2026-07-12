import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import DateRangePicker from './DateRangePicker.svelte'

describe('DateRangePicker', () => {
  it('renders explicit instant values and resolves presets once', async () => {
    render(DateRangePicker, {
      props: {
        startValue: new Date('2026-06-15T00:00:00.000Z'),
        endValue: new Date('2026-06-16T00:00:00.000Z'),
        presetAnchor: new Date('2026-06-16T12:00:00.000Z'),
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
    render(DateRangePicker, {
      props: {
        startValue: new Date('2026-06-15T13:00:00.000Z'),
        endValue: new Date('2026-06-15T12:00:00.000Z'),
        min: new Date('2026-06-15T11:30:00.000Z'),
        max: new Date('2026-06-15T12:30:00.000Z'),
        outOfBoundsMessage: 'Range must stay within the selected availability window.',
        showValidation: true,
      },
    })

    expect(screen.getByRole('alert')).toHaveTextContent('Start must be earlier than end.')
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Range must stay within the selected availability window.',
    )
  })
})
