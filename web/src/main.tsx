import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

export function App() {
  return (
    <main>
      <h1>CList</h1>
    </main>
  )
}

const container = typeof document === 'undefined' ? null : document.getElementById('root')

if (container) {
  createRoot(container).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}
