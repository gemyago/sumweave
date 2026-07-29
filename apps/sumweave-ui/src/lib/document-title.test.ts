import { describe, expect, it } from 'vitest'
import { documentTitle } from './document-title'

describe('documentTitle', () => {
  it('formats the current page before its optional section and product name', () => {
    expect(documentTitle('Accounts', 'Finance')).toBe('Accounts · Finance · Sumweave')
  })

  it('uses the product name as the fallback title', () => {
    expect(documentTitle()).toBe('Sumweave')
  })
})
