import { useState } from 'react'
import { listEntries } from '../api'
import { Card, Code, EntryTable, HeadersTable, KeyValues, Tabs } from '../components'
import { formatDuration, formatTime, prettyBody, statusClass } from '../format'
import { useFetch } from '../hooks'
import { sections } from '../sections'
import type { Entry, RequestContent } from '../types'

export function RequestDetail({ entry }: { entry: Entry<RequestContent> }) {
  const c = entry.content
  const { data: related = [] } = useFetch(() => listEntries({ requestId: entry.id, limit: 200 }), [entry.id])
  const relatedSections = sections
    .filter((s) => s.type !== 'request')
    .map((s) => ({ section: s, entries: related.filter((e) => e.type === s.type) }))

  const tabs = [
    { key: 'request', label: 'Request' },
    { key: 'response', label: 'Response' },
    ...relatedSections.map((r) => ({ key: r.section.path, label: `${r.section.title} (${r.entries.length})` })),
  ]
  const [tab, setTab] = useState('request')
  const activeRelated = relatedSections.find((r) => r.section.path === tab)

  return (
    <>
      <h2>
        <span className="method">{c.method}</span> <span className="mono">{c.path}</span>
      </h2>
      <Card>
        <KeyValues
          items={[
            ['Status', <span className={statusClass(c.status)}>{c.status}</span>],
            ['Duration', formatDuration(c.durationMs)],
            ['Time', formatTime(entry.createdAt)],
            ['IP', c.ip],
            !!c.query && ['Query', <span className="mono">{c.query}</span>],
          ]}
        />
      </Card>

      <Tabs tabs={tabs} active={tab} onChange={setTab} />

      {tab === 'request' && (
        <>
          <Card title="Headers">
            <HeadersTable headers={c.headers} />
          </Card>
          <Card title="Body">
            <Code>{prettyBody(c.body)}</Code>
          </Card>
        </>
      )}
      {tab === 'response' && (
        <>
          <Card title="Headers">
            <HeadersTable headers={c.responseHeaders} />
          </Card>
          <Card title="Body">
            <Code>{prettyBody(c.responseBody)}</Code>
          </Card>
        </>
      )}
      {activeRelated && <EntryTable section={activeRelated.section} entries={activeRelated.entries} />}
    </>
  )
}
