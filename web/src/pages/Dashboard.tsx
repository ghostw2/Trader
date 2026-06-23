import { useState } from 'react'
import type { FormEvent } from 'react'
import { useMarketStream } from '../hooks/useMarketStream'
import { useAlerts } from '../hooks/useAlerts'
import { Chart } from '../components/Chart'

export function Dashboard() {
  const wsUrl = import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080/ws'
  const { tick, connected } = useMarketStream(wsUrl)
  const { alerts, createAlert, deleteAlert } = useAlerts()
  const [targetPrice, setTargetPrice] = useState('')
  const [direction, setDirection] = useState<'above' | 'below'>('below')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const price = parseFloat(targetPrice)
    if (!isNaN(price) && price > 0) {
      await createAlert('BTCUSDT', price, direction)
      setTargetPrice('')
    }
  }

  return (
    <div style={{ padding: '1.5rem', fontFamily: 'monospace', background: '#0f0f1a', minHeight: '100vh', color: '#e0e0e0' }}>
      <h1 style={{ marginBottom: '0.5rem' }}>Trader</h1>
      <p style={{ color: connected ? '#00d4aa' : '#ff4444', marginBottom: '1rem' }}>
        {connected ? '● Connected' : '○ Disconnected'}
      </p>

      {tick && (
        <div style={{ marginBottom: '1.5rem' }}>
          <div style={{ fontSize: '0.9rem', opacity: 0.6 }}>{tick.symbol}</div>
          <div style={{ fontSize: '2.5rem', fontWeight: 'bold' }}>
            ${parseFloat(tick.price).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
          </div>
        </div>
      )}

      <Chart tick={tick} />

      <div style={{ marginTop: '2rem' }}>
        <h2 style={{ fontSize: '1rem', marginBottom: '1rem' }}>Price Alerts</h2>

        <form onSubmit={handleSubmit} style={{ display: 'flex', gap: '0.5rem', marginBottom: '1.5rem', flexWrap: 'wrap' }}>
          <select
            value={direction}
            onChange={e => setDirection(e.target.value as 'above' | 'below')}
            style={{ background: '#1a1a2e', color: '#e0e0e0', border: '1px solid #2a2a4a', padding: '0.4rem 0.6rem' }}
          >
            <option value="below">Below</option>
            <option value="above">Above</option>
          </select>
          <input
            type="number"
            placeholder="Target price (USD)"
            value={targetPrice}
            onChange={e => setTargetPrice(e.target.value)}
            style={{ background: '#1a1a2e', color: '#e0e0e0', border: '1px solid #2a2a4a', padding: '0.4rem 0.6rem', width: '200px' }}
          />
          <button
            type="submit"
            style={{ background: '#00d4aa', color: '#0f0f1a', border: 'none', padding: '0.4rem 1rem', cursor: 'pointer', fontFamily: 'monospace', fontWeight: 'bold' }}
          >
            Set Alert
          </button>
        </form>

        {alerts.length === 0 ? (
          <p style={{ opacity: 0.4 }}>No alerts. Set one above.</p>
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse', maxWidth: '520px' }}>
            <thead>
              <tr style={{ opacity: 0.5, textAlign: 'left', fontSize: '0.85rem' }}>
                <th style={{ paddingBottom: '0.5rem' }}>Direction</th>
                <th style={{ paddingBottom: '0.5rem' }}>Target</th>
                <th style={{ paddingBottom: '0.5rem' }}>Status</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {alerts.map(a => (
                <tr key={a.id} style={{ borderTop: '1px solid #2a2a4a' }}>
                  <td style={{ padding: '0.5rem 0.5rem 0.5rem 0' }}>{a.direction}</td>
                  <td style={{ padding: '0.5rem' }}>
                    ${a.target_price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </td>
                  <td style={{ padding: '0.5rem', color: a.triggered_at ? '#00d4aa' : '#e0e0e0' }}>
                    {a.triggered_at ? '✓ Triggered' : '◉ Watching'}
                  </td>
                  <td style={{ padding: '0.5rem 0' }}>
                    <button
                      onClick={() => deleteAlert(a.id)}
                      style={{ background: 'none', border: 'none', color: '#ff4444', cursor: 'pointer', fontFamily: 'monospace', fontSize: '1rem' }}
                    >
                      ✕
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
