import { useState, useEffect, useCallback } from 'react'

export interface Alert {
  id: number
  symbol: string
  target_price: number
  direction: 'above' | 'below'
  created_at: number
  triggered_at: number | null
}

export function useAlerts() {
  const [alerts, setAlerts] = useState<Alert[]>([])

  const fetchAlerts = useCallback(async () => {
    const res = await fetch('/api/alerts')
    if (res.ok) setAlerts(await res.json())
  }, [])

  useEffect(() => {
    fetchAlerts()
    const id = setInterval(fetchAlerts, 5000)
    return () => clearInterval(id)
  }, [fetchAlerts])

  const createAlert = useCallback(async (
    symbol: string,
    targetPrice: number,
    direction: 'above' | 'below',
  ) => {
    const res = await fetch('/api/alerts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ symbol, target_price: targetPrice, direction }),
    })
    if (res.ok) await fetchAlerts()
  }, [fetchAlerts])

  const deleteAlert = useCallback(async (id: number) => {
    await fetch(`/api/alerts/${id}`, { method: 'DELETE' })
    await fetchAlerts()
  }, [fetchAlerts])

  return { alerts, createAlert, deleteAlert }
}
