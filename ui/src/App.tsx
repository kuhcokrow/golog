import { clearEntries } from './api'
import { useHashPath } from './hooks'
import { DetailPage } from './pages/DetailPage'
import { ListPage } from './pages/ListPage'
import { sections } from './sections'

export function App() {
  const [, name, id] = useHashPath().split('/')
  const section = sections.find((s) => s.path === name) ?? sections[0]

  const onClear = async () => {
    if (!confirm('Delete all entries?')) return
    await clearEntries()
    window.location.hash = `/${section.path}`
    window.location.reload()
  }

  return (
    <div className="layout">
      <aside>
        <h1>Golog</h1>
        <nav>
          {sections.map((s) => (
            <a key={s.path} href={`#/${s.path}`} className={s === section ? 'active' : ''}>
              {s.title}
            </a>
          ))}
        </nav>
        <button className="danger" onClick={onClear}>
          Clear entries
        </button>
      </aside>
      <main>{id ? <DetailPage key={id} id={id} /> : <ListPage key={section.path} section={section} />}</main>
    </div>
  )
}
