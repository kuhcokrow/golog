import type { Entry, EntryType } from './types'

export interface ListParams {
  type?: EntryType
  requestId?: string
  search?: string
  from?: string
  to?: string
  limit?: number
  offset?: number
}

// Fired when the session is missing or expired, so the app can show the login page.
export const unauthorizedEvent = 'golog:unauthorized'

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init)
  if (res.status === 401 && !url.startsWith('api/auth')) window.dispatchEvent(new Event(unauthorizedEvent))
  if (!res.ok) throw new Error((await res.text()).trim() || `${res.status} ${res.statusText}`)
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

export interface Settings {
  // Retention in seconds. 0 means no time limit, null means the store does not support it.
  retention: number | null
}

export function getSettings(): Promise<Settings> {
  return request('api/settings')
}

export function updateSettings(settings: Settings): Promise<Settings> {
  return request('api/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  })
}

export type AuthState = 'setup' | 'login' | 'ok'

export interface AuthStatus {
  enabled: boolean
  state: AuthState
  username?: string
}

function postJSON<T>(url: string, body?: unknown): Promise<T> {
  return request(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
}

export function getAuth(): Promise<AuthStatus> {
  return request('api/auth')
}

export function register(username: string, password: string, confirmPassword: string): Promise<AuthStatus> {
  return postJSON('api/auth/register', { username, password, confirmPassword })
}

export function login(username: string, password: string): Promise<AuthStatus> {
  return postJSON('api/auth/login', { username, password })
}

export function logout(): Promise<void> {
  return postJSON('api/auth/logout')
}
