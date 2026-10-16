import { useState } from 'react'
import { listEntries } from '../api'
import { EntryTable } from '../components'
import { useDebounced, useFetch } from '../hooks'
import type { Section } from '../sections'

const pageSize = 50

// datetime-local only has minute precision, so "to" covers the whole minute.
function toISO(local: string, endOfMinute = false): string | undefined {
  if (!local) return undefined
  const t = new Date(local).getTime() + (endOfMinute ? 59_999 : 0)
  return new Date(t).toISOString()
}

export function ListPage({ section }: { section: Section }) {
  const [page, setPage] = useState(0)
  const [search, setSearch] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const debouncedSearch = useDebounced(search, 300)

  const { data, error } = useFetch(
    () =>
      listEntries({
        type: section.type,
        search: debouncedSearch,
        from: toISO(from),
        to: toISO(to, true),
        limit: pageSize,
        offset: page * pageSize,
      }),
    [section.type, page, debouncedSearch, from, to],
    page === 0 ? 2000 : 0,
  )

  const update = (set: (v: string) => void) => (v: string) => {
    set(v)
    setPage(0)
  }
  const reset = () => {
    setSearch('')
    setFrom('')
    setTo('')
    setPage(0)
  }

  return (
    <>
      <h2>{section.title}</h2>
      <div className="toolbar">
        <input
          type="search"
          className="search"
          placeholder="Search..."
          value={search}
          onChange={(e) => update(setSearch)(e.target.value)}
        />
        <label>
          From
          <input type="datetime-local" value={from} onChange={(e) => update(setFrom)(e.target.value)} />
        </label>
        <label>
          To
          <input type="datetime-local" value={to} onChange={(e) => update(setTo)(e.target.value)} />
        </label>
        <button onClick={reset} disabled={!search && !from && !to}>
          Reset
        </button>
        <div className="pager">
          <button disabled={page === 0} onClick={() => setPage(page - 1)}>
            Newer
          </button>
          <button disabled={!data || data.length < pageSize} onClick={() => setPage(page + 1)}>
            Older
          </button>
        </div>
      </div>
      {error && <p className="error">{error}</p>}
      {data && <EntryTable section={section} entries={data} />}
    </>
  )
}
