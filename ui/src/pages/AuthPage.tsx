import { useState, type FormEvent } from 'react'
import { login, register, type AuthStatus } from '../api'

const minPassword = 8

export function AuthPage({ mode, onDone }: { mode: 'setup' | 'login'; onDone: (s: AuthStatus) => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState<string>()
  const [busy, setBusy] = useState(false)
  const isSetup = mode === 'setup'

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError(undefined)
    if (isSetup && password.length < minPassword) return setError(`Password must be at least ${minPassword} characters.`)
    if (isSetup && password !== confirm) return setError('Passwords do not match.')

    setBusy(true)
    try {
      onDone(isSetup ? await register(username, password, confirm) : await login(username, password))
    } catch (err) {
      setError((err as Error).message)
      setBusy(false)
    }
  }

  return (
    <div className="auth">
      <form className="card auth-card" onSubmit={submit}>
        <h1>Golog</h1>
        <p className="hint">
          {isSetup ? 'Create the dashboard account. Only one account can exist.' : 'Log in to open the dashboard.'}
        </p>
        <label>
          Username
          <input autoFocus required autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} />
        </label>
        <label>
          Password
          <input
            required
            type="password"
            autoComplete={isSetup ? 'new-password' : 'current-password'}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
        {isSetup && (
          <label>
            Confirm password
            <input
              required
              type="password"
              autoComplete="new-password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
            />
          </label>
        )}
        {error && <p className="error">{error}</p>}
        <button type="submit" className="primary" disabled={busy}>
          {busy ? 'Please wait...' : isSetup ? 'Create account' : 'Log in'}
        </button>
      </form>
    </div>
  )
}
