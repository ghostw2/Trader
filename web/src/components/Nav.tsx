type Page = 'dashboard' | 'portfolio' | 'strategy'

interface NavProps {
  active: Page
  onChange: (page: Page) => void
}

export function Nav({ active, onChange }: NavProps) {
  return (
    <nav style={{ display: 'flex', gap: '1.5rem', marginBottom: '1.5rem' }}>
      {(['dashboard', 'portfolio', 'strategy'] as const).map(page => (
        <button
          key={page}
          onClick={() => onChange(page)}
          style={{
            background: 'none',
            border: 'none',
            borderBottom: active === page ? '2px solid #00d4aa' : '2px solid transparent',
            color: active === page ? '#00d4aa' : '#e0e0e0',
            fontFamily: 'monospace',
            fontSize: '0.95rem',
            cursor: 'pointer',
            padding: '0.25rem 0',
          }}
        >
          {page.charAt(0).toUpperCase() + page.slice(1)}
        </button>
      ))}
    </nav>
  )
}
