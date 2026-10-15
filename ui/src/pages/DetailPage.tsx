import { getEntry } from '../api'
import { Card, Code, KeyValues } from '../components'
import { formatDuration, formatTime, levelClass, prettyJSON } from '../format'
import { useFetch } from '../hooks'
import type { CacheContent, Entry, ExceptionContent, LogContent, QueryContent, RequestContent } from '../types'
import { RequestDetail } from './RequestDetail'

export function DetailPage({ id }: { id: string }) {
  const { data, error } = useFetch(() => getEntry(id), [id])

  if (error) return <p className="error">{error}</p>
  if (!data) return <p className="muted">Loading...</p>
  if (data.type === 'request') return <RequestDetail entry={data as Entry<RequestContent>} />

  return (
    <>
      <h2>{data.type.charAt(0).toUpperCase() + data.type.slice(1)}</h2>
      <Card>
        <KeyValues
          items={[
            ['Time', formatTime(data.createdAt)],
            ['Request', data.requestId ? <a href={`#/requests/${data.requestId}`}>{data.requestId}</a> : '-'],
          ]}
        />
      </Card>
      <EntryBody entry={data} />
    </>
  )
}

export function EntryBody({ entry }: { entry: Entry }) {
  switch (entry.type) {
    case 'query': {
      const c = entry.content as QueryContent
      return (
        <>
          <Card title="SQL">
            <Code>{c.sql}</Code>
          </Card>
          <Card>
            <KeyValues
              items={[
                ['Duration', formatDuration(c.durationMs)],
                ['Bindings', <span className="mono">{prettyJSON(c.bindings)}</span>],
                !!c.error && ['Error', <span className="error">{c.error}</span>],
              ]}
            />
          </Card>
        </>
      )
    }
    case 'log': {
      const c = entry.content as LogContent
      return (
        <Card>
          <KeyValues
            items={[
              ['Level', <span className={levelClass(c.level)}>{c.level}</span>],
              ['Message', c.message],
            ]}
          />
          <h3>Attributes</h3>
          <Code>{c.attrs ? prettyJSON(c.attrs) : ''}</Code>
        </Card>
      )
    }
    case 'exception': {
      const c = entry.content as ExceptionContent
      return (
        <>
          <Card title="Message">
            <p className="error">{c.message}</p>
          </Card>
          <Card title="Stack trace">
            <Code>{c.stack}</Code>
          </Card>
        </>
      )
    }
    case 'cache': {
      const c = entry.content as CacheContent
      return (
        <Card>
          <KeyValues
            items={[
              ['Action', <span className="badge gray">{c.action}</span>],
              ['Key', <span className="mono">{c.key}</span>],
              ['Duration', formatDuration(c.durationMs)],
            ]}
          />
          <h3>Value</h3>
          <Code>{c.value === undefined ? '' : prettyJSON(c.value)}</Code>
        </Card>
      )
    }
    default:
      return <Code>{prettyJSON(entry.content)}</Code>
  }
}
