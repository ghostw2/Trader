import { useEffect, useRef } from 'react'
import { createChart, LineSeries } from 'lightweight-charts'
import type { ISeriesApi, UTCTimestamp } from 'lightweight-charts'
import type { Tick } from '../hooks/useMarketStream'

interface ChartProps {
  tick: Tick | null
}

export function Chart({ tick }: ChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const seriesRef = useRef<ISeriesApi<'Line'> | null>(null)

  useEffect(() => {
    if (!containerRef.current) return
    const chart = createChart(containerRef.current, {
      width: containerRef.current.clientWidth,
      height: 300,
      layout: { background: { color: '#1a1a2e' }, textColor: '#e0e0e0' },
      grid: { vertLines: { color: '#2a2a4a' }, horzLines: { color: '#2a2a4a' } },
    })
    const series = chart.addSeries(LineSeries, { color: '#00d4aa', lineWidth: 2 })
    seriesRef.current = series

    const handleResize = () => {
      if (containerRef.current) chart.applyOptions({ width: containerRef.current.clientWidth })
    }
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      chart.remove()
    }
  }, [])

  useEffect(() => {
    if (!tick || !seriesRef.current) return
    try {
      seriesRef.current.update({
        time: Math.floor(tick.timestamp / 1000) as UTCTimestamp, // ms → seconds
        value: parseFloat(tick.price),
      })
    } catch {
      // lightweight-charts throws on duplicate/out-of-order timestamps
    }
  }, [tick])

  return <div ref={containerRef} style={{ width: '100%' }} />
}
