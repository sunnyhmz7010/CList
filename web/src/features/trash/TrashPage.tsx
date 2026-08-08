import {useQuery, useQueryClient} from '@tanstack/react-query'

import {apiClient} from '../../api/client'

export interface TrashItem { id: string; name: string; type?: string; item_id?: string }
interface Batch { id: string; root_type: string; root_id: string; items: TrashItem[] }

export function TrashPage({items}: {items?: TrashItem[]}) {
  const queryClient = useQueryClient()
  const query = useQuery({queryKey: ['trash'], queryFn: () => apiClient<{items: Batch[]}>('/api/v1/trash'), enabled: items === undefined})
  const batches = items ? [{id: 'preview', root_type: 'file', root_id: '', items}] : query.data?.items ?? []
  async function restore(id: string) {
    await apiClient(`/api/v1/trash/${id}/restore`, {method: 'POST'})
    await queryClient.invalidateQueries({queryKey: ['trash']})
  }
  async function purge(id: string) {
    if (!window.confirm('彻底删除后无法恢复，确认继续？')) return
    await apiClient(`/api/v1/trash/${id}`, {method: 'DELETE'})
    await queryClient.invalidateQueries({queryKey: ['trash']})
  }
  return <main className="trash-page"><h1>回收站</h1>
    <p>回收站阶段不会删除 Telegram 消息或本地对象。</p>
    {batches.map(batch => <section key={batch.id} className="trash-batch">
      <ul>{batch.items.map(item => <li key={item.id ?? item.item_id}>{item.name ?? item.original_name}</li>)}</ul>
      <button type="button" onClick={() => void restore(batch.id)}>恢复</button>
      <button type="button" onClick={() => void purge(batch.id)}>彻底删除</button>
      <p>彻底删除后无法恢复</p>
    </section>)}
  </main>
}
