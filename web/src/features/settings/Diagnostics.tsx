import {useQuery} from '@tanstack/react-query'

import {apiClient} from '../../api/client'

interface DiagnosticsResult { ready: {ok: boolean; checks: Record<string, string>}; pending_jobs: number }

export function Diagnostics() {
  const query = useQuery({queryKey: ['diagnostics'], queryFn: () => apiClient<DiagnosticsResult>('/api/v1/admin/diagnostics'), refetchInterval: 30_000})
  return <section className="settings-card"><h2>运行诊断</h2>
    {query.isError && <p role="alert">诊断信息加载失败</p>}
    <dl>{Object.entries(query.data?.ready.checks ?? {}).map(([name, status]) => <div key={name}><dt>{name}</dt><dd>{status}</dd></div>)}</dl>
    <p>未完成任务：{query.data?.pending_jobs ?? 0}</p>
  </section>
}
