export function formatTime(iso: string): string {
  return new Date(iso).toLocaleString()
}

export function formatDuration(ms?: number): string {
  if (ms === undefined) return '-'
  return ms >= 1000 ? `${(ms / 1000).toFixed(2)} s` : `${ms.toFixed(2)} ms`
}

export function prettyBody(body?: string): string {
  if (!body) return ''
  try {
    return JSON.stringify(JSON.parse(body), null, 2)
  } catch {
    return body
  }
}

export function prettyJSON(v: unknown): string {
  return JSON.stringify(v, null, 2)
}

export function statusClass(status: number): string {
  if (status >= 500) return 'badge red'
  if (status >= 400) return 'badge yellow'
  return 'badge green'
}

export function levelClass(level: string): string {
  if (level.startsWith('ERROR')) return 'badge red'
  if (level.startsWith('WARN')) return 'badge yellow'
  if (level.startsWith('DEBUG')) return 'badge gray'
  return 'badge blue'
}
