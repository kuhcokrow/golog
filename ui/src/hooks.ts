import { useEffect, useState } from 'react'

export function useHashPath(): string {
  const read = () => window.location.hash.slice(1) || '/requests'
  const [path, setPath] = useState(read)
  useEffect(() => {
    const onChange = () => setPath(read())
    window.addEventListener('hashchange', onChange)
    return () => window.removeEventListener('hashchange', onChange)
  }, [])
  return path
}

export function useFetch<T>(load: () => Promise<T>, deps: unknown[], pollMs = 0) {
  const [data, setData] = useState<T>()
  const [error, setError] = useState<string>()

  useEffect(() => {
    let active = true
    const run = () =>
      load()
        .then((d) => active && (setData(d), setError(undefined)))
        .catch((e: Error) => active && setError(e.message))
    run()
    const timer = pollMs > 0 ? setInterval(run, pollMs) : undefined
    return () => {
      active = false
      clearInterval(timer)
    }
  }, deps)

  return { data, error }
}

export function useDebounced<T>(value: T, ms: number): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), ms)
    return () => clearTimeout(timer)
  }, [value, ms])
  return debounced
}
