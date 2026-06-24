import { useState } from 'react'
import { Dashboard } from './pages/Dashboard'
import { Portfolio } from './pages/Portfolio'
import { Strategy } from './pages/Strategy'
import { Nav } from './components/Nav'

type Page = 'dashboard' | 'portfolio' | 'strategy'

function App() {
  const [page, setPage] = useState<Page>('dashboard')

  return (
    <div
      style={{
        padding: '1.5rem',
        fontFamily: 'monospace',
        background: '#0f0f1a',
        minHeight: '100vh',
        color: '#e0e0e0',
      }}
    >
      <h1 style={{ marginBottom: '1rem' }}>Trader</h1>
      <Nav active={page} onChange={setPage} />
      {page === 'dashboard' && <Dashboard />}
      {page === 'portfolio' && <Portfolio />}
      {page === 'strategy' && <Strategy />}
    </div>
  )
}

export default App
