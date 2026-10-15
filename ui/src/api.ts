import type { Entry, EntryType } from './types'

export interface ListParams {
  type?: EntryType
  requestId?: string
  limit?: number
  offset?: number
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.status === 204 ? (undefined as T) : res.json()
}

export function listEntries(params: ListParams): Promise<Entry[]> {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') q.set(k, String(v))
  }
  return request(`api/entries?${q}`)
}

export function getEntry(id: string): Promise<Entry> {
  return request(`api/entries/${id}`)
}

export function clearEntries(): Promise<void> {
  return request('api/entries', { method: 'DELETE' })
}
