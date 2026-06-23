import { useState } from 'react'
import { Dashboard } from './pages/Dashboard'
import { Portfolio } from './pages/Portfolio'
import { Nav } from './components/Nav'

type Page = 'dashboard' | 'portfolio'

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
      {page === 'dashboard' ? <Dashboard /> : <Portfolio />}
    </div>
  )
}

export default App
