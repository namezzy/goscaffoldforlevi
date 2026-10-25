import { useCallback, useEffect, useRef, useState } from 'react'
import { getRecentRequests, streamRequests } from '../api/requests'
import type { RequestEvent } from '../api/types'

export const MAX_ITEMS = 300
const RECONNECT_DELAY_MS = 1500
// A request/s rate computed over a window this short is noisy on a quiet
// dashboard; long enough smooths it without making the number feel stale.
export const RATE_WINDOW_MS = 10_000

export type ConnectionStatus = 'connecting' | 'live' | 'reconnecting'

export interface RequestFeed {
  items: RequestEvent[]
  status: ConnectionStatus
  rateCount: number
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/**
 * Subscribes to the live request stream. Hoisted out of the feed table so
 * that the table and the path map read one subscription instead of opening
 * two streams and disagreeing about what has arrived.
 */
export function useRequestFeed(): RequestFeed {
  const [items, setItems] = useState<RequestEvent[]>([])
  const [status, setStatus] = useState<ConnectionStatus>('connecting')

  // The rate is tracked from its own uncapped list of timestamps, not by
  // counting inside `items`. `items` is capped at MAX_ITEMS for display, so
  // under real traffic (more than MAX_ITEMS arriving inside RATE_WINDOW_MS)
  // counting there silently ceilinged the displayed rate at MAX_ITEMS /
  // (RATE_WINDOW_MS / 1000) — about 30 req/s at these constants — no matter
  // how much traffic was actually arriving.
  const timestampsRef = useRef<number[]>([])
  const [rateCount, setRateCount] = useState(0)

  const pruneAndCountRate = useCallback(() => {
    const cutoff = Date.now() - RATE_WINDOW_MS
    timestampsRef.current = timestampsRef.current.filter((t) => t > cutoff)
    setRateCount(timestampsRef.current.length)
  }, [])

  useEffect(() => {
    let cancelled = false
    const controller = new AbortController()

    async function loop() {
      try {
        const page = await getRecentRequests()
        if (cancelled) return
        setItems(page.items.slice(-MAX_ITEMS).reverse())
        const cutoff = Date.now() - RATE_WINDOW_MS
        timestampsRef.current = page.items
          .map((e) => new Date(e.time).getTime())
          .filter((t) => t > cutoff)
        setRateCount(timestampsRef.current.length)
      } catch {
        // The live stream below still gets a chance to connect.
      }

      while (!cancelled) {
        try {
          await streamRequests((e) => {
            if (cancelled) return
            setStatus('live')
            setItems((prev) => [e, ...prev].slice(0, MAX_ITEMS))
            timestampsRef.current.push(new Date(e.time).getTime())
            pruneAndCountRate()
          }, controller.signal)
        } catch {
          // Aborted (unmount) or a genuine network failure — either way the
          // reconnect loop below is the single place that decides what
          // happens next.
        }
        if (cancelled) return
        setStatus('reconnecting')
        await sleep(RECONNECT_DELAY_MS)
      }
    }

    void loop()
    // The rate must keep decaying even while nothing new arrives — without
    // its own tick, a burst that stopped ten seconds ago would still read
    // as "3.0 req/s" forever, because nothing else re-evaluates the window.
    const rateTimer = setInterval(pruneAndCountRate, 1000)
    return () => {
      cancelled = true
      controller.abort()
      clearInterval(rateTimer)
    }
  }, [pruneAndCountRate])

  return { items, status, rateCount }
}
