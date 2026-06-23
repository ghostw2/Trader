import { useState, useEffect, useCallback } from 'react'

export interface PortfolioState {
  cash_balance: number
  btc_balance: number
  avg_buy_price: number
  current_price: number
  total_value: number
  unrealized_pl: number
}

export interface Trade {
  id: number
  side: 'buy' | 'sell'
  quantity: number
  price: number
  total: number
  created_at: number
}

export function usePortfolio() {
  const [portfolio, setPortfolio] = useState<PortfolioState | null>(null)
  const [trades, setTrades] = useState<Trade[]>([])

  const fetchPortfolio = useCallback(async () => {
    try {
      const res = await fetch('/api/portfolio')
      if (res.ok) setPortfolio(await res.json())
    } catch (err) {
      console.error('Failed to fetch portfolio:', err)
    }
  }, [])

  const fetchTrades = useCallback(async () => {
    try {
      const res = await fetch('/api/trades')
      if (res.ok) setTrades(await res.json())
    } catch (err) {
      console.error('Failed to fetch trades:', err)
    }
  }, [])

  useEffect(() => {
    fetchPortfolio()
    fetchTrades()
    const id = setInterval(() => {
      fetchPortfolio()
      fetchTrades()
    }, 3000)
    return () => clearInterval(id)
  }, [fetchPortfolio, fetchTrades])

  const placeOrder = useCallback(async (side: 'buy' | 'sell', quantity: number): Promise<void> => {
    const res = await fetch('/api/orders', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ side, quantity }),
    })
    if (!res.ok) {
      const text = await res.text()
      throw new Error(text.trim())
    }
    await fetchPortfolio()
    await fetchTrades()
  }, [fetchPortfolio, fetchTrades])

  return { portfolio, trades, placeOrder }
}
