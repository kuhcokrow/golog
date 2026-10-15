import { useState } from 'react'
import { listEntries } from '../api'
import { EntryTable } from '../components'
import { useFetch } from '../hooks'
import type { Section } from '../sections'

const pageSize = 50

export function ListPage({ section }: { section: Section }) {
  const [page, setPage] = useState(0)
  const { data, error } = useFetch(
    () => listEntries({ type: section.type, limit: pageSize, offset: page * pageSize }),
    [section.type, page],
    page === 0 ? 2000 : 0,
  )

  return (
    <>
      <h2>{section.title}</h2>
      {error && <p className="error">{error}</p>}
      {data && <EntryTable section={section} entries={data} />}
      <div className="pager">
        <button disabled={page === 0} onClick={() => setPage(page - 1)}>
          Newer
        </button>
        <button disabled={!data || data.length < pageSize} onClick={() => setPage(page + 1)}>
          Older
        </button>
      </div>
    </>
  )
}
