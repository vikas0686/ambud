import { useEffect, useState } from 'react'

interface PollingState<T> {
  data: T | null
  error: string | null
  loading: boolean
}

// Polls fetcher every intervalMs, starting immediately. fetcher must be
// a stable reference (a module-level function, not an inline closure)
// — it's a dependency of the effect below, so a fresh closure on every
// render would restart the interval every render instead of polling on
// a fixed cadence. See web/src/api/client.ts's exports for the
// intended call pattern: usePolling(listNodes, 3000).
export function usePolling<T>(fetcher: () => Promise<T>, intervalMs: number): PollingState<T> {
  const [state, setState] = useState<PollingState<T>>({ data: null, error: null, loading: true })

  useEffect(() => {
    let cancelled = false

    const poll = async () => {
      try {
        const data = await fetcher()
        if (!cancelled) setState({ data, error: null, loading: false })
      } catch (err) {
        if (!cancelled) {
          setState((s) => ({
            ...s,
            error: err instanceof Error ? err.message : String(err),
            loading: false,
          }))
        }
      }
    }

    void poll()
    const id = setInterval(() => void poll(), intervalMs)
    return () => {
      cancelled = true
      clearInterval(id)
    }
  }, [fetcher, intervalMs])

  return state
}
