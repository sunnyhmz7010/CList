import {useQuery} from '@tanstack/react-query'

import {apiClient} from '../../api/client'
import {FileAccessDialog} from './FileAccessDialog'

interface APIFile { public_id: string; file_name: string; size: number; mime_type: string; gallery_visibility: Visibility }
export interface FileSummary { publicId: string; fileName: string; size: number; mimeType?: string }
type Visibility = 'inherit' | 'visible' | 'hidden'
type ManagedFile = FileSummary & {galleryVisibility?: Visibility}

export function FileList({files}: {files?: FileSummary[]}) {
  const query = useQuery({queryKey: ['files'], queryFn: () => apiClient<{items: APIFile[]}>('/api/v1/files'), enabled: files === undefined})
  const items: ManagedFile[] = files ?? query.data?.items.map(file => ({publicId: file.public_id, fileName: file.file_name, size: file.size, mimeType: file.mime_type, galleryVisibility: file.gallery_visibility})) ?? []
  return <section aria-labelledby="files-title">
    <h2 id="files-title">文件</h2>
    {query.isError && <p role="alert">文件列表加载失败</p>}
    <ul className="file-list">{items.map(file => <li key={file.publicId}>
      <a href={`/f/${file.publicId}/${encodeURIComponent(file.fileName)}`}>{file.fileName}</a>
      <span>{formatSize(file.size)}</span>
      <progress max="100" value="0" aria-label={`${file.fileName} 上传进度`} />
      <FileAccessDialog publicId={file.publicId} />
      {file.galleryVisibility && <select aria-label={`${file.fileName} 相册可见性`} value={file.galleryVisibility} onChange={event => void apiClient(`/api/v1/files/${file.publicId}`, {method: 'PATCH', body: JSON.stringify({gallery_visibility: event.target.value})}).then(() => query.refetch())}>
        <option value="inherit">继承</option><option value="visible">显示</option><option value="hidden">隐藏</option>
      </select>}
    </li>)}</ul>
  </section>
}

function formatSize(size: number) {
  if (size < 1024) return `${size} B`
  if (size < 1024 ** 2) return `${(size / 1024).toFixed(1)} KiB`
  return `${(size / 1024 ** 2).toFixed(1)} MiB`
}
