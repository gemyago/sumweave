import { beforeEach, describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import ThemeSegmentedControl from './ThemeSegmentedControl.svelte'
import { themeStore } from '../lib/theme/theme-store.svelte'

describe('ThemeSegmentedControl', () => {
  beforeEach(() => {
    localStorage.clear()
    themeStore.setPreference('auto')
  })

  it('wraps backward from auto to dark on ArrowLeft', async () => {
    const user = userEvent.setup()
    render(ThemeSegmentedControl)

    const auto = screen.getByRole('radio', { name: 'Auto' })
    auto.focus()
    await user.keyboard('{ArrowLeft}')

    expect(screen.getByRole('radio', { name: 'Dark' })).toHaveAttribute('aria-checked', 'true')
  })
})
