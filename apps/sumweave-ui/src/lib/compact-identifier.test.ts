import { describe, expect, it } from 'vitest'
import { formatCompactIdentifier } from './compact-identifier'

describe('compact identifiers', () => {
  it('preserves blank and short identifiers', () => {
    expect(formatCompactIdentifier('   ')).toBe('')
    expect(formatCompactIdentifier('job-1')).toBe('job-1')
  })

  it('uses supplied bounds when abbreviating a long identifier', () => {
    expect(formatCompactIdentifier('abcdefghijklmnop', { start: 3, end: 4 })).toBe('abc...mnop')
  })
})
