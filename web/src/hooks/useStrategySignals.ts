import { useState, useEffect } from 'react'

export interface Signal {
  side: 'BUY' | 'SELL' | 'HOLD'
  sma_fast: number
  sma_slow: number
  ema: number
  rsi: number
  price: number
  timestamp: number
}

export function useStrategySignals(): {
  latestSignal: Signal | null
  recentSignals: Signal[]
  connected: boolean
} {
  const [latestSignal, setLatestSignal] = useState<Signal | null>(null)
  const [recentSignals, setRecentSignals] = useState<Signal[]>([])
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    const es = new EventSource('/api/strategy/signals')

    es.onopen = () => setConnected(true)

    es.onmessage = (e: MessageEvent) => {
      try {
        const sig = JSON.parse(e.data) as Signal
        setLatestSignal(sig)
        setRecentSignals(prev => [sig, ...prev].slice(0, 20))
      } catch {
        // ignore malformed events
      }
    }

    es.onerror = () => setConnected(false)

    return () => {
      es.close()
      setConnected(false)
    }
  }, [])

  return { latestSignal, recentSignals, connected }
}
