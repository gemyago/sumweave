import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import FinancePager from './FinancePager.svelte'
import FinancePagerHarness from './FinancePager.harness.svelte'

describe('FinancePager', () => {
  it('uses buttons with accessible status and delegates pager callbacks', async () => {
    const user = userEvent.setup()
    const onPrevious = vi.fn(() => true)
    const onNext = vi.fn(() => true)
    render(FinancePager, {
      label: 'Ledger pages', status: 'Page 2', controls: 'ledger-results', hasPrevious: true, hasNext: true, onPrevious, onNext,
    })

    expect(screen.getByRole('status')).toHaveTextContent('Page 2')
    expect(screen.getByRole('navigation', { name: 'Ledger pages' })).toHaveAttribute('tabindex', '-1')
    expect(screen.getByRole('button', { name: 'Ledger pages: previous page' })).toHaveAttribute('type', 'button')
    expect(screen.getByRole('button', { name: 'Ledger pages: next page' })).toHaveAttribute('aria-controls', 'ledger-results')
    await user.click(screen.getByRole('button', { name: 'Ledger pages: previous page' }))
    await user.click(screen.getByRole('button', { name: 'Ledger pages: next page' }))
    expect(onPrevious).toHaveBeenCalledOnce()
    expect(onNext).toHaveBeenCalledOnce()
  })

  it('focuses the successful pager region without scrolling', async () => {
    const user = userEvent.setup()
    const focus = vi.spyOn(HTMLElement.prototype, 'focus')
    render(FinancePager, {
      label: 'Ledger pages', status: 'Page 1', controls: 'ledger-results', hasNext: true,
      onPrevious: () => false, onNext: async () => true,
    })

    await user.click(screen.getByRole('button', { name: 'Ledger pages: next page' }))

    expect(focus).toHaveBeenCalledWith({ preventScroll: true })
    expect(screen.getByRole('navigation', { name: 'Ledger pages' })).toHaveFocus()
    focus.mockRestore()
  })

  it('does not move focus after a failed page action and disables controls while busy', () => {
    render(FinancePager, {
      label: 'Ledger pages', status: 'Loading page 2', controls: 'ledger-results', busy: true, hasPrevious: true, hasNext: true, onPrevious: () => false, onNext: () => false,
    })

    expect(screen.getByRole('navigation', { name: 'Ledger pages' })).toHaveAttribute('aria-busy', 'true')
    expect(screen.getByRole('button', { name: 'Ledger pages: previous page' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Ledger pages: next page' })).toBeDisabled()
  })

  it('keeps its viewport anchor and focuses its stable region when a full page becomes final', async () => {
    const user = userEvent.setup()
    const scrollBy = vi.spyOn(window, 'scrollBy').mockImplementation(() => undefined)
    render(FinancePagerHarness)
    const pager = screen.getByRole('navigation', { name: 'Ledger pages' })
    const previousNext = screen.getByRole('button', { name: 'Ledger pages: next page' })
    const rect = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      return { top: this === pager && screen.getByRole('status').textContent === 'Page 1' ? 700 : 200 } as DOMRect
    })

    await user.click(previousNext)

    await waitFor(() => expect(scrollBy).toHaveBeenCalledWith(0, -500))
    expect(screen.getByRole('navigation', { name: 'Ledger pages' })).toBe(pager)
    expect(pager).toHaveFocus()
    expect(screen.getByRole('button', { name: 'Ledger pages: next page' })).toBeDisabled()
    rect.mockRestore()
    scrollBy.mockRestore()
  })

  it('does not compensate or restore focus after a failed page action', async () => {
    const user = userEvent.setup()
    const scrollBy = vi.spyOn(window, 'scrollBy').mockImplementation(() => undefined)
    const focus = vi.spyOn(HTMLElement.prototype, 'focus')
    render(FinancePagerHarness, { succeed: false })

    await user.click(screen.getByRole('button', { name: 'Ledger pages: next page' }))

    expect(scrollBy).not.toHaveBeenCalled()
    expect(focus).not.toHaveBeenCalledWith({ preventScroll: true })
    focus.mockRestore()
    scrollBy.mockRestore()
  })
})
