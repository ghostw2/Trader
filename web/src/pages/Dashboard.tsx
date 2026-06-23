import { useMarketStream } from '../hooks/useMarketStream'
import { Chart } from '../components/Chart'

export function Dashboard() {
  const { tick, connected } = useMarketStream('ws://localhost:8080/ws')

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
    </div>
  )
}
