import type { ReactNode } from 'react'
import type { Entry } from './types'
import type { Section } from './sections'
import { formatTime } from './format'

export function EntryTable({ section, entries }: { section: Section; entries: Entry[] }) {
  if (entries.length === 0) return <p className="empty">No entries yet.</p>
  return (
    <table>
      <thead>
        <tr>
          {section.columns.map((c) => (
            <th key={c.title} className={c.className}>
              {c.title}
            </th>
          ))}
          <th>Time</th>
        </tr>
      </thead>
      <tbody>
        {entries.map((e) => (
          <tr key={e.id} onClick={() => (window.location.hash = `/${section.path}/${e.id}`)}>
            {section.columns.map((c) => (
              <td key={c.title} className={c.className}>
                {c.render(e)}
              </td>
            ))}
            <td className="muted nowrap">{formatTime(e.createdAt)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

export function Card({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <section className="card">
      {title && <h3>{title}</h3>}
      {children}
    </section>
  )
}

export type KeyValue = [string, ReactNode]

export function KeyValues({ items }: { items: (KeyValue | false | undefined)[] }) {
  return (
    <dl className="kv">
      {items.filter((i): i is KeyValue => !!i).map(([k, v]) => (
        <div key={k}>
          <dt>{k}</dt>
          <dd>{v}</dd>
        </div>
      ))}
    </dl>
  )
}

export function Code({ children }: { children: string }) {
  return <pre className="code">{children || <span className="muted">(empty)</span>}</pre>
}

export function HeadersTable({ headers }: { headers: Record<string, string[]> }) {
  const keys = Object.keys(headers ?? {}).sort()
  if (keys.length === 0) return <p className="muted">(none)</p>
  return <KeyValues items={keys.map((k): KeyValue => [k, <span className="mono">{headers[k].join(', ')}</span>])} />
}

export interface Tab {
  key: string
  label: string
}

export function Tabs({ tabs, active, onChange }: { tabs: Tab[]; active: string; onChange: (key: string) => void }) {
  return (
    <div className="tabs">
      {tabs.map((t) => (
        <button key={t.key} className={t.key === active ? 'active' : ''} onClick={() => onChange(t.key)}>
          {t.label}
        </button>
      ))}
    </div>
  )
}
