import type { ReactNode } from 'react'
import type { CacheContent, Entry, EntryType, ExceptionContent, LogContent, QueryContent, RequestContent } from './types'
import { formatDuration, levelClass, statusClass } from './format'

export interface Column {
  title: string
  render: (e: Entry) => ReactNode
  className?: string
}

export interface Section {
  path: string
  title: string
  type: EntryType
  columns: Column[]
}

export const sections: Section[] = [
  {
    path: 'requests',
    title: 'Requests',
    type: 'request',
    columns: [
      { title: 'Method', render: (e) => <span className="method">{(e.content as RequestContent).method}</span> },
      { title: 'Path', className: 'grow mono', render: (e) => (e.content as RequestContent).path },
      {
        title: 'Status',
        render: (e) => {
          const s = (e.content as RequestContent).status
          return <span className={statusClass(s)}>{s}</span>
        },
      },
      { title: 'Duration', render: (e) => formatDuration((e.content as RequestContent).durationMs) },
    ],
  },
  {
    path: 'queries',
    title: 'Queries',
    type: 'query',
    columns: [
      {
        title: 'SQL',
        className: 'grow mono truncate',
        render: (e) => {
          const c = e.content as QueryContent
          return (
            <>
              {c.error && <span className="badge red">error</span>} {c.sql}
            </>
          )
        },
      },
      { title: 'Duration', render: (e) => formatDuration((e.content as QueryContent).durationMs) },
    ],
  },
  {
    path: 'logs',
    title: 'Logs',
    type: 'log',
    columns: [
      {
        title: 'Level',
        render: (e) => {
          const l = (e.content as LogContent).level
          return <span className={levelClass(l)}>{l}</span>
        },
      },
      { title: 'Message', className: 'grow truncate', render: (e) => (e.content as LogContent).message },
    ],
  },
  {
    path: 'exceptions',
    title: 'Exceptions',
    type: 'exception',
    columns: [{ title: 'Message', className: 'grow truncate', render: (e) => (e.content as ExceptionContent).message }],
  },
  {
    path: 'cache',
    title: 'Cache',
    type: 'cache',
    columns: [
      { title: 'Action', render: (e) => <span className="badge gray">{(e.content as CacheContent).action}</span> },
      { title: 'Key', className: 'grow mono truncate', render: (e) => (e.content as CacheContent).key },
    ],
  },
]

export function sectionFor(type: EntryType): Section {
  return sections.find((s) => s.type === type)!
}
