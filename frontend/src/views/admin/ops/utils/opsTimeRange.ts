export interface OpsISOTimeRange {
  start_time: string
  end_time: string
}

export interface OpsLocalTimeRange {
  start: string
  end: string
}

function padDatePart(value: number): string {
  return String(value).padStart(2, '0')
}

// datetime-local expects wall-clock components, not a UTC ISO string. Using
// toISOString().slice(...) shifts the displayed range in non-UTC time zones.
export function formatOpsDatetimeLocal(date: Date): string {
  if (!Number.isFinite(date.getTime())) return ''
  return [
    date.getFullYear(),
    '-',
    padDatePart(date.getMonth() + 1),
    '-',
    padDatePart(date.getDate()),
    'T',
    padDatePart(date.getHours()),
    ':',
    padDatePart(date.getMinutes())
  ].join('')
}

export function createOpsLocalTimeRange(windowMs: number, now = new Date()): OpsLocalTimeRange {
  const safeWindowMs = Number.isFinite(windowMs) && windowMs >= 0 ? windowMs : 0
  const end = new Date(now.getTime())
  const start = new Date(end.getTime() - safeWindowMs)
  return {
    start: formatOpsDatetimeLocal(start),
    end: formatOpsDatetimeLocal(end)
  }
}

export function buildOpsISOTimeRange(startValue: string, endValue: string): OpsISOTimeRange | null {
  if (!startValue || !endValue) return null
  const start = new Date(startValue)
  const end = new Date(endValue)
  if (!Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime()) || start > end) return null
  return {
    start_time: start.toISOString(),
    end_time: end.toISOString()
  }
}
