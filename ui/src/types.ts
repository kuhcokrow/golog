export type EntryType = 'request' | 'query' | 'log' | 'exception' | 'cache'

export interface Entry<T = unknown> {
  id: string
  type: EntryType
  requestId?: string
  createdAt: string
  content: T
}

export type Headers = Record<string, string[]>

export interface RequestContent {
  method: string
  path: string
  query?: string
  ip: string
  status: number
  durationMs: number
  headers: Headers
  body?: string
  responseHeaders: Headers
  responseBody?: string
}

export interface QueryContent {
  sql: string
  bindings: unknown[]
  durationMs: number
  error?: string
}

export interface LogContent {
  level: string
  message: string
  attrs?: Record<string, unknown>
}

export interface ExceptionContent {
  message: string
  stack: string
}

export interface CacheContent {
  action: string
  key: string
  value?: unknown
  durationMs?: number
}
