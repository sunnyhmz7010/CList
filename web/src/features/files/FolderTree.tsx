import {useQuery} from '@tanstack/react-query'

import {apiClient} from '../../api/client'

interface Folder { id: string; name: string }

export function FolderTree() {
  const query = useQuery({queryKey: ['folders'], queryFn: () => apiClient<{items: Folder[]}>('/api/v1/folders')})
  return <nav aria-label="文件夹"><h2>文件夹</h2><ul>{query.data?.items.map(folder => <li key={folder.id}>{folder.name}</li>)}</ul></nav>
}
