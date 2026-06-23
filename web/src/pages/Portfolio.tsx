import { useState } from 'react'
import type { FormEvent } from 'react'
import { usePortfolio } from '../hooks/usePortfolio'

export function Portfolio() {
  const { portfolio, trades, placeOrder } = usePortfolio()
  const [side, setSide] = useState<'buy' | 'sell'>('buy')
  const [quantity, setQuantity] = useState('')
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const qty = parseFloat(quantity)
    if (isNaN(qty) || qty <= 0) return
    setError(null)
    try {
      await placeOrder(side, qty)
      setQuantity('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Order failed')
    }
  }

  const pl = portfolio?.unrealized_pl ?? 0
  const plColor = pl >= 0 ? '#00d4aa' : '#ff4444'
  const fmt = (n: number) =>
    n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })

  return (
    <div>
      {portfolio && (
        <div style={{ display: 'flex', gap: '2rem', flexWrap: 'wrap', marginBottom: '2rem' }}>
          <Stat label="Portfolio Value" value={`$${fmt(portfolio.total_value)}`} />
          <Stat label="Cash" value={`$${fmt(portfolio.cash_balance)}`} />
          <Stat label="BTC Held" value={`${portfolio.btc_balance.toFixed(6)}`} />
          <Stat
            label="Avg Buy Price"
            value={portfolio.avg_buy_price > 0 ? `$${fmt(portfolio.avg_buy_price)}` : '—'}
          />
          <Stat
            label="Unrealized P&L"
            value={`${pl >= 0 ? '+' : ''}$${fmt(pl)}`}
            color={plColor}
          />
          <Stat label="Current Price" value={`$${fmt(portfolio.current_price)}`} />
        </div>
      )}

      <form
        onSubmit={handleSubmit}
        style={{ display: 'flex', gap: '0.5rem', marginBottom: '2rem', flexWrap: 'wrap', alignItems: 'center' }}
      >
        <select
          value={side}
          onChange={e => setSide(e.target.value as 'buy' | 'sell')}
          style={{ background: '#1a1a2e', color: '#e0e0e0', border: '1px solid #2a2a4a', padding: '0.4rem 0.6rem' }}
        >
          <option value="buy">Buy BTC</option>
          <option value="sell">Sell BTC</option>
        </select>
        <input
          type="number"
          placeholder="Quantity (BTC)"
          value={quantity}
          onChange={e => setQuantity(e.target.value)}
          step="0.0001"
          min="0"
          style={{
            background: '#1a1a2e',
            color: '#e0e0e0',
            border: '1px solid #2a2a4a',
            padding: '0.4rem 0.6rem',
            width: '180px',
          }}
        />
        <button
          type="submit"
          style={{
            background: side === 'buy' ? '#00d4aa' : '#ff4444',
            color: '#0f0f1a',
            border: 'none',
            padding: '0.4rem 1rem',
            cursor: 'pointer',
            fontFamily: 'monospace',
            fontWeight: 'bold',
          }}
        >
          {side === 'buy' ? 'Buy' : 'Sell'}
        </button>
        {error && <span style={{ color: '#ff4444', fontSize: '0.875rem' }}>{error}</span>}
      </form>

      <h2 style={{ fontSize: '0.9rem', opacity: 0.5, marginBottom: '0.75rem' }}>Trade History</h2>
      {trades.length === 0 ? (
        <p style={{ opacity: 0.4 }}>No trades yet.</p>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse', maxWidth: '600px', fontSize: '0.9rem' }}>
          <thead>
            <tr style={{ opacity: 0.5, textAlign: 'left' }}>
              <th style={{ paddingBottom: '0.5rem' }}>Side</th>
              <th style={{ paddingBottom: '0.5rem' }}>Quantity</th>
              <th style={{ paddingBottom: '0.5rem' }}>Price</th>
              <th style={{ paddingBottom: '0.5rem' }}>Total</th>
            </tr>
          </thead>
          <tbody>
            {trades.map(t => (
              <tr key={t.id} style={{ borderTop: '1px solid #2a2a4a' }}>
                <td
                  style={{
                    padding: '0.4rem 0.5rem 0.4rem 0',
                    color: t.side === 'buy' ? '#00d4aa' : '#ff4444',
                  }}
                >
                  {t.side}
                </td>
                <td style={{ padding: '0.4rem 0.5rem' }}>{t.quantity.toFixed(6)}</td>
                <td style={{ padding: '0.4rem 0.5rem' }}>${fmt(t.price)}</td>
                <td style={{ padding: '0.4rem 0.5rem' }}>${fmt(t.total)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

function Stat({ label, value, color = '#e0e0e0' }: { label: string; value: string; color?: string }) {
  return (
    <div>
      <div style={{ fontSize: '0.75rem', opacity: 0.5, marginBottom: '0.2rem' }}>{label}</div>
      <div style={{ fontSize: '1.25rem', fontWeight: 'bold', color }}>{value}</div>
    </div>
  )
}
