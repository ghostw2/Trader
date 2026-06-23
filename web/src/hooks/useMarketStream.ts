import { useEffect, useRef, useState } from 'react'

export interface Tick {
  symbol: string
  price: string
  timestamp: number
}

export function useMarketStream(url: string): { tick: Tick | null; connected: boolean } {
  const [tick, setTick] = useState<Tick | null>(null)
  const [connected, setConnected] = useState(false)
  const retryRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    let ws: WebSocket
    let cancelled = false

    function connect() {
      ws = new WebSocket(url)

      ws.onopen = () => { if (!cancelled) setConnected(true) }

      ws.onclose = () => {
        if (!cancelled) {
          setConnected(false)
          retryRef.current = setTimeout(connect, 2000)
        }
      }

      ws.onerror = () => ws.close()

      ws.onmessage = (e: MessageEvent<string>) => {
        if (!cancelled) setTick(JSON.parse(e.data) as Tick)
      }
    }

    connect()

    return () => {
      cancelled = true
      if (retryRef.current) clearTimeout(retryRef.current)
      ws?.close()
    }
  }, [url])

  return { tick, connected }
}
