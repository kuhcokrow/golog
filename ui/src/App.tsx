import { useEffect, useState } from 'react'
import { clearEntries, getAuth, logout, unauthorizedEvent, type AuthStatus } from './api'
import { useHashPath } from './hooks'
import { AuthPage } from './pages/AuthPage'
import { DetailPage } from './pages/DetailPage'
import { ListPage } from './pages/ListPage'
import { SettingsPage } from './pages/SettingsPage'
import { sections } from './sections'

export function App() {
  const [auth, setAuth] = useState<AuthStatus>()
  const [authError, setAuthError] = useState<string>()

  useEffect(() => {
    const load = () =>
      getAuth()
        .then(setAuth)
        .catch((e: Error) => setAuthError(e.message))
    load()
    window.addEventListener(unauthorizedEvent, load)
    return () => window.removeEventListener(unauthorizedEvent, load)
  }, [])

  if (authError) return <p className="error page-message">{authError}</p>
  if (!auth) return <p className="muted page-message">Loading...</p>
  if (auth.state !== 'ok') return <AuthPage key={auth.state} mode={auth.state} onDone={setAuth} />

  const onLogout = async () => {
    await logout()
    setAuth({ ...auth, state: 'login', username: undefined })
  }

  return <Dashboard auth={auth} onLogout={onLogout} />
}

function Dashboard({ auth, onLogout }: { auth: AuthStatus; onLogout: () => void }) {
  const [, name, id] = useHashPath().split('/')
  const isSettings = name === 'settings'
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
            <a key={s.path} href={`#/${s.path}`} className={!isSettings && s === section ? 'active' : ''}>
              {s.title}
            </a>
          ))}
        </nav>
        <a href="#/settings" className={`nav-link ${isSettings ? 'active' : ''}`}>
          Settings
        </a>
        <button className="danger" onClick={onClear}>
          Clear entries
        </button>
        {auth.enabled && (
          <div className="account">
            <span className="muted truncate">{auth.username}</span>
            <button onClick={onLogout}>Log out</button>
          </div>
        )}
      </aside>
      <main>
        {isSettings ? (
          <SettingsPage />
        ) : id ? (
          <DetailPage key={id} id={id} />
        ) : (
          <ListPage key={section.path} section={section} />
        )}
      </main>
    </div>
  )
}
