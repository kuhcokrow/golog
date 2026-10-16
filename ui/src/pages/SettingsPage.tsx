import { useEffect, useState } from 'react'
import { getSettings, updateSettings } from '../api'
import { Card } from '../components'

const hour = 3600
const day = 24 * hour

const presets: [number, string][] = [
  [0, 'No limit'],
  [hour, '1 hour'],
  [6 * hour, '6 hours'],
  [day, '24 hours'],
  [3 * day, '3 days'],
  [7 * day, '7 days'],
  [30 * day, '30 days'],
]

function label(seconds: number): string {
  const preset = presets.find(([s]) => s === seconds)
  if (preset) return preset[1]
  if (seconds % day === 0) return `${seconds / day} days`
  if (seconds % hour === 0) return `${seconds / hour} hours`
  return `${Math.round(seconds / 60)} minutes`
}

export function SettingsPage() {
  const [saved, setSaved] = useState<number | null>()
  const [value, setValue] = useState(0)
  const [status, setStatus] = useState<{ ok: boolean; text: string }>()
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    getSettings()
      .then((s) => {
        setSaved(s.retention)
        setValue(s.retention ?? 0)
      })
      .catch((e: Error) => setStatus({ ok: false, text: e.message }))
  }, [])

  const save = async () => {
    setSaving(true)
    setStatus(undefined)
    try {
      const s = await updateSettings({ retention: value })
      setSaved(s.retention)
      setStatus({ ok: true, text: 'Saved.' })
    } catch (e) {
      setStatus({ ok: false, text: (e as Error).message })
    } finally {
      setSaving(false)
    }
  }

  // Keep a saved value that is not a preset (e.g. set from code) selectable.
  const options = saved != null && !presets.some(([s]) => s === saved) ? [...presets, [saved, label(saved)] as [number, string]] : presets

  return (
    <>
      <h2>Settings</h2>
      <Card title="Retention">
        {saved === undefined && !status && <p className="muted">Loading...</p>}
        {saved === null && <p className="muted">The configured store does not support changing retention.</p>}
        {saved != null && (
          <div className="field">
            <p className="hint">Entries older than this are deleted. Changes apply immediately.</p>
            <div className="row">
              <select value={value} onChange={(e) => setValue(Number(e.target.value))}>
                {options.map(([s, l]) => (
                  <option key={s} value={s}>
                    {l}
                  </option>
                ))}
              </select>
              <button onClick={save} disabled={saving || value === saved}>
                {saving ? 'Saving...' : 'Save'}
              </button>
            </div>
          </div>
        )}
        {status && <p className={status.ok ? 'success' : 'error'}>{status.text}</p>}
      </Card>
    </>
  )
}
