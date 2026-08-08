import type {FormEvent, ReactNode} from 'react'
import {useState} from 'react'
import {useQuery} from '@tanstack/react-query'

import {apiClient} from '../../api/client'

export interface AdminStatus {
  initialized: boolean
  authenticated: boolean
}

export function AdminGate({children}: {children: ReactNode}) {
  const status = useQuery({queryKey: ['admin-status'], queryFn: () => apiClient<AdminStatus>('/api/v1/admin/status')})
  if (status.isLoading) return <main><p>正在检查登录状态</p></main>
  if (status.isError || !status.data) return <main><p role="alert">登录状态检查失败</p></main>
  return <AuthLanding status={status.data} onAuthenticated={() => void status.refetch()}>{children}</AuthLanding>
}

export function AuthLanding({status, children, onAuthenticated}: {status: AdminStatus; children?: ReactNode; onAuthenticated?: () => void}) {
  if (!status.initialized) return <AdminCredentialsForm title="初始化管理员" submitLabel="初始化并进入" initialize onAuthenticated={onAuthenticated} />
  if (!status.authenticated) return <AdminCredentialsForm title="管理员登录" submitLabel="登录" onAuthenticated={onAuthenticated} />
  return <>{children}</>
}

function AdminCredentialsForm({title, submitLabel, initialize = false, onAuthenticated}: {title: string; submitLabel: string; initialize?: boolean; onAuthenticated?: () => void}) {
  const [account, setAccount] = useState('')
  const [password, setPassword] = useState('')
  const [message, setMessage] = useState('')
  const [pending, setPending] = useState(false)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPending(true)
    setMessage('')
    try {
      const body = JSON.stringify({account, password})
      if (initialize) await apiClient('/api/v1/admin/initialize', {method: 'POST', body})
      await apiClient('/api/v1/admin/login', {method: 'POST', body})
      onAuthenticated?.()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '请求失败')
    } finally {
      setPending(false)
    }
  }

  return <main className="auth-page">
    <form onSubmit={event => void submit(event)}>
      <h2>{title}</h2>
      {initialize && <p>首次使用请设置管理员账号和密码。</p>}
      <label>账号<input value={account} onChange={event => setAccount(event.target.value)} autoComplete="username" required /></label>
      <label>密码<input type="password" value={password} onChange={event => setPassword(event.target.value)} autoComplete={initialize ? 'new-password' : 'current-password'} required /></label>
      <button type="submit" disabled={pending}>{pending ? '处理中' : submitLabel}</button>
      {message && <p role="alert">{message}</p>}
    </form>
  </main>
}
