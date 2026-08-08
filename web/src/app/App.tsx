import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {RouterProvider} from 'react-router-dom'

import {createAppRouter} from './router'
export {AdminLayout} from './AdminLayout'
export {AuthLanding} from '../features/auth/AdminGate'

const queryClient = new QueryClient()
const router = typeof document === 'undefined' ? null : createAppRouter()

export function Shell() {
  return <header className="app-header"><h1>CList</h1><button type="button">切换主题</button></header>
}

export function App() {
  return <QueryClientProvider client={queryClient}>{router && <RouterProvider router={router} />}</QueryClientProvider>
}
