import { describe, expect, it } from 'vitest'
import { buildOpsISOTimeRange, createOpsLocalTimeRange, formatOpsDatetimeLocal } from '../opsTimeRange'

describe('opsTimeRange', () => {
  it('formats datetime-local values from local wall-clock components', () => {
    const date = new Date(2026, 6, 15, 22, 32, 45)
    expect(formatOpsDatetimeLocal(date)).toBe('2026-07-15T22:32')
  })

  it('creates both boundaries from the same clock snapshot', () => {
    const now = new Date(2026, 6, 15, 22, 32, 45)
    expect(createOpsLocalTimeRange(24 * 60 * 60 * 1000, now)).toEqual({
      start: '2026-07-14T22:32',
      end: '2026-07-15T22:32'
    })
  })

  it('rejects incomplete, invalid, and reversed ranges before an API request', () => {
    expect(buildOpsISOTimeRange('', '2026-07-15T22:32')).toBeNull()
    expect(buildOpsISOTimeRange('invalid', '2026-07-15T22:32')).toBeNull()
    expect(buildOpsISOTimeRange('2026-07-16T22:32', '2026-07-15T22:32')).toBeNull()
  })

  it('converts a valid local range to ordered RFC3339 values', () => {
    const range = buildOpsISOTimeRange('2026-07-14T22:32', '2026-07-15T22:32')
    expect(range).not.toBeNull()
    expect(Date.parse(range!.start_time)).toBeLessThanOrEqual(Date.parse(range!.end_time))
  })
})
