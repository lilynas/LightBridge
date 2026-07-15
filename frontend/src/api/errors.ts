export interface ApiErrorOptions {
  status?: number
  code?: unknown
  reason?: unknown
  message?: string
  detail?: unknown
  metadata?: unknown
  data?: unknown
  url?: string
  method?: string
  response?: unknown
  cause?: unknown
}

/**
 * Error returned by the API layer. It remains an actual Error instance while
 * preserving every structured field returned by the backend for UI display and
 * troubleshooting.
 */
export class ApiError extends Error {
  readonly status: number
  readonly code?: unknown
  readonly reason?: unknown
  readonly detail?: unknown
  readonly metadata?: unknown
  readonly data?: unknown
  readonly url?: string
  readonly method?: string
  readonly response?: unknown
  readonly cause?: unknown

  constructor(options: ApiErrorOptions) {
    super(options.message || String(options.detail || '') || 'Unknown API error')
    this.name = 'ApiError'
    this.status = options.status ?? 0
    this.code = options.code
    this.reason = options.reason
    this.detail = options.detail
    this.metadata = options.metadata
    this.data = options.data
    this.url = options.url
    this.method = options.method
    this.response = options.response
    this.cause = options.cause
  }
}

export function getApiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim()) return error.message
  if (error && typeof error === 'object') {
    const value = error as Record<string, unknown>
    for (const candidate of [value.message, value.detail, value.reason, value.code]) {
      if (typeof candidate === 'string' && candidate.trim()) return candidate.trim()
    }
  }
  return fallback
}

