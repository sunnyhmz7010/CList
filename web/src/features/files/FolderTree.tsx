import {useQuery} from '@tanstack/react-query'

import {apiClient} from '../../api/client'

interface Folder { id: string; name: string; gallery_visibility: 'inherit' | 'visible' | 'hidden' }

export function FolderTree() {
  const query = useQuery({queryKey: ['folders'], queryFn: () => apiClient<{items: Folder[]}>('/api/v1/folders')})
  return <nav aria-label="文件夹"><h2>文件夹</h2><ul>{query.data?.items.map(folder => <li key={folder.id}>{folder.name}<select aria-label={`${folder.name} 相册可见性`} value={folder.gallery_visibility} onChange={event => void apiClient(`/api/v1/folders/${folder.id}`, {method: 'PATCH', body: JSON.stringify({gallery_visibility: event.target.value})}).then(() => query.refetch())}><option value="inherit">继承</option><option value="visible">显示</option><option value="hidden">隐藏</option></select></li>)}</ul></nav>
}
