import {useState, type ReactNode} from 'react'

import {apiClient} from '../../api/client'

export function GuestGate({children, scope = 'home'}: {children: ReactNode; scope?: 'home' | 'gallery'}) {
  const [allowed, setAllowed] = useState(false)
  const [password, setPassword] = useState('')
  if (allowed) return <>{children}</>
  return <main><h2>{scope === 'gallery' ? '访问相册' : '访问 CList'}</h2><form onSubmit={event => { event.preventDefault(); void apiClient(`/api/v1/guest/${scope}/session`, {method: 'POST', body: JSON.stringify({password})}).then(() => setAllowed(true)) }}>
    <label>访问密码<input type="password" value={password} onChange={event => setPassword(event.target.value)} /></label><button>进入</button>
  </form></main>
}
