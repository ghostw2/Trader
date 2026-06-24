import { useState } from 'react'
import { useStrategySignals } from '../hooks/useStrategySignals'
import type { Signal } from '../hooks/useStrategySignals'

interface BacktestTrade {
  side: string
  price: number
  time: number
}

interface BacktestSummary {
  trades: BacktestTrade[]
  total_trades: number
  final_value: number
  return_pct: number
}

const fmt = (n: number) =>
  n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })

function signalColor(side: Signal['side']): string {
  if (side === 'BUY') return '#00d4aa'
  if (side === 'SELL') return '#ff4444'
  return '#e0e0e0'
}

export function Strategy() {
  const { latestSignal, recentSignals, connected } = useStrategySignals()
  const [backtest, setBacktest] = useState<BacktestSummary | null>(null)
  const [loading, setLoading] = useState(false)
  const [backtestError, setBacktestError] = useState<string | null>(null)

  async function runBacktest() {
    setLoading(true)
    setBacktestError(null)
    try {
      const resp = await fetch('/api/strategy/backtest', { method: 'POST' })
      if (!resp.ok) {
        const text = await resp.text()
        throw new Error(text.trim() || 'Backtest failed')
      }
      setBacktest(await resp.json())
    } catch (err) {
      setBacktestError(err instanceof Error ? err.message : 'Backtest failed')
    } finally {
      setLoading(false)
    }
  }

  const returnColor = backtest && backtest.return_pct >= 0 ? '#00d4aa' : '#ff4444'

  return (
    <div>
      {/* Live indicators */}
      <p style={{ color: connected ? '#00d4aa' : '#ff4444', marginBottom: '1rem' }}>
        {connected ? '● Connected' : '○ Disconnected'}
      </p>

      {latestSignal && (
        <div style={{ display: 'flex', gap: '2rem', flexWrap: 'wrap', marginBottom: '2rem' }}>
          <Stat label="Signal" value={latestSignal.side} color={signalColor(latestSignal.side)} />
          <Stat label="Price" value={`$${fmt(latestSignal.price)}`} />
          <Stat label="SMA(10)" value={fmt(latestSignal.sma_fast)} />
          <Stat label="SMA(50)" value={fmt(latestSignal.sma_slow)} />
          <Stat label="EMA(20)" value={fmt(latestSignal.ema)} />
          <Stat label="RSI(14)" value={latestSignal.rsi.toFixed(1)} />
        </div>
      )}

      {/* Recent signals table */}
      {recentSignals.length > 0 && (
        <div style={{ marginBottom: '2rem' }}>
          <h2 style={{ fontSize: '0.9rem', opacity: 0.5, marginBottom: '0.75rem' }}>
            Recent Signals
          </h2>
          <table style={{ width: '100%', borderCollapse: 'collapse', maxWidth: '640px', fontSize: '0.875rem' }}>
            <thead>
              <tr style={{ opacity: 0.5, textAlign: 'left' }}>
                <th style={{ paddingBottom: '0.5rem' }}>Time</th>
                <th style={{ paddingBottom: '0.5rem' }}>Side</th>
                <th style={{ paddingBottom: '0.5rem' }}>Price</th>
                <th style={{ paddingBottom: '0.5rem' }}>SMA(10)</th>
                <th style={{ paddingBottom: '0.5rem' }}>SMA(50)</th>
              </tr>
            </thead>
            <tbody>
              {recentSignals.map((s, i) => (
                <tr key={i} style={{ borderTop: '1px solid #2a2a4a' }}>
                  <td style={{ padding: '0.35rem 0.5rem 0.35rem 0', opacity: 0.5 }}>
                    {new Date(s.timestamp).toLocaleTimeString()}
                  </td>
                  <td style={{ padding: '0.35rem 0.5rem', color: signalColor(s.side), fontWeight: 'bold' }}>
                    {s.side}
                  </td>
                  <td style={{ padding: '0.35rem 0.5rem' }}>${fmt(s.price)}</td>
                  <td style={{ padding: '0.35rem 0.5rem' }}>{fmt(s.sma_fast)}</td>
                  <td style={{ padding: '0.35rem 0.5rem' }}>{fmt(s.sma_slow)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Backtest panel */}
      <div>
        <h2 style={{ fontSize: '0.9rem', opacity: 0.5, marginBottom: '0.75rem' }}>Backtest</h2>
        <p style={{ fontSize: '0.8rem', opacity: 0.4, marginBottom: '0.75rem' }}>
          30 days of 5-minute BTCUSDT candles, SMA(10/50) crossover strategy
        </p>
        <button
          onClick={runBacktest}
          disabled={loading}
          style={{
            background: loading ? '#2a2a4a' : '#00d4aa',
            color: loading ? '#e0e0e0' : '#0f0f1a',
            border: 'none',
            padding: '0.4rem 1rem',
            cursor: loading ? 'default' : 'pointer',
            fontFamily: 'monospace',
            fontWeight: 'bold',
            marginBottom: '1rem',
          }}
        >
          {loading ? 'Running...' : 'Run Backtest'}
        </button>

        {backtestError && (
          <p style={{ color: '#ff4444', fontSize: '0.875rem', marginBottom: '1rem' }}>
            {backtestError}
          </p>
        )}

        {backtest && (
          <>
            <div style={{ display: 'flex', gap: '2rem', flexWrap: 'wrap', marginBottom: '1.5rem' }}>
              <Stat label="Total Trades" value={String(backtest.total_trades)} />
              <Stat label="Final Value" value={`$${fmt(backtest.final_value)}`} />
              <Stat
                label="Return"
                value={`${backtest.return_pct >= 0 ? '+' : ''}${backtest.return_pct.toFixed(2)}%`}
                color={returnColor}
              />
            </div>

            {backtest.trades.length > 0 && (
              <div style={{ maxHeight: '300px', overflowY: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', maxWidth: '480px', fontSize: '0.875rem' }}>
                  <thead>
                    <tr style={{ opacity: 0.5, textAlign: 'left' }}>
                      <th style={{ paddingBottom: '0.5rem' }}>Index</th>
                      <th style={{ paddingBottom: '0.5rem' }}>Side</th>
                      <th style={{ paddingBottom: '0.5rem' }}>Price</th>
                    </tr>
                  </thead>
                  <tbody>
                    {backtest.trades.map((tr, i) => (
                      <tr key={i} style={{ borderTop: '1px solid #2a2a4a' }}>
                        <td style={{ padding: '0.35rem 0.5rem 0.35rem 0', opacity: 0.5 }}>{tr.time}</td>
                        <td style={{ padding: '0.35rem 0.5rem', color: tr.side === 'BUY' ? '#00d4aa' : '#ff4444' }}>
                          {tr.side}
                        </td>
                        <td style={{ padding: '0.35rem 0.5rem' }}>${fmt(tr.price)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            {backtest.total_trades === 0 && (
              <p style={{ opacity: 0.4 }}>No crossover signals in this period.</p>
            )}
          </>
        )}
      </div>
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
