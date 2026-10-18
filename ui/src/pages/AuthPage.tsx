import { useState, type FormEvent } from 'react'
import { login, register, type AuthStatus } from '../api'

const minPassword = 8

function EyeIcon({ open }: { open: boolean }) {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z" />
      <circle cx="12" cy="12" r="3" />
      {!open && <line x1="3" y1="3" x2="21" y2="21" />}
    </svg>
  )
}

function PasswordInput({
  value,
  onChange,
  autoComplete,
}: {
  value: string
  onChange: (v: string) => void
  autoComplete: string
}) {
  const [visible, setVisible] = useState(false)
  return (
    <div className="password-input">
      <input
        required
        type={visible ? 'text' : 'password'}
        autoComplete={autoComplete}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      <button
        type="button"
        className="icon-button"
        aria-label={visible ? 'Hide password' : 'Show password'}
        title={visible ? 'Hide password' : 'Show password'}
        onClick={() => setVisible(!visible)}
      >
        <EyeIcon open={!visible} />
      </button>
    </div>
  )
}

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
          <PasswordInput
            autoComplete={isSetup ? 'new-password' : 'current-password'}
            value={password}
            onChange={setPassword}
          />
        </label>
        {isSetup && (
          <label>
            Confirm password
            <PasswordInput autoComplete="new-password" value={confirm} onChange={setConfirm} />
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
