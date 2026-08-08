import {useMemo, useState} from 'react'
import {useQuery} from '@tanstack/react-query'

import {apiClient} from '../../api/client'
import {Previewer, type PreviewModel} from '../../components/Previewer'

interface GalleryFile {
  public_id: string
  folder_id?: string
  file_name: string
  mime_type: string
  created_at: string
}

export function Gallery() {
  const [type, setType] = useState('')
  const [name, setName] = useState('')
  const [folder, setFolder] = useState('')
  const [selected, setSelected] = useState<PreviewModel>()
  const queryString = useMemo(() => new URLSearchParams({type, name, folder}).toString(), [type, name, folder])
  const gallery = useQuery({queryKey: ['gallery', queryString], queryFn: () => apiClient<{items: GalleryFile[]}>(`/api/v1/gallery?${queryString}`)})
  return <main className="gallery-page">
    <h1>相册</h1>
    <form className="gallery-filters" onSubmit={event => event.preventDefault()}>
      <label>类型<select value={type} onChange={event => setType(event.target.value)}><option value="">全部</option><option value="image/">图片</option><option value="video/">视频</option><option value="audio/">音频</option><option value="application/pdf">PDF</option><option value="text/plain">TXT</option></select></label>
      <label>名称<input value={name} onChange={event => setName(event.target.value)} /></label>
      <label>文件夹<input value={folder} onChange={event => setFolder(event.target.value)} /></label>
    </form>
    {gallery.isError && <p role="alert">相册加载失败</p>}
    <ul className="gallery-grid">{gallery.data?.items.map(file => <li key={file.public_id}>
      <button type="button" onClick={() => setSelected(toPreview(file))}>{file.file_name}</button>
      <time dateTime={file.created_at}>{new Date(file.created_at).toLocaleString()}</time>
    </li>)}</ul>
    {selected && <section className="preview-panel"><button type="button" onClick={() => setSelected(undefined)}>关闭预览</button><Previewer preview={selected} /></section>}
  </main>
}

function toPreview(file: GalleryFile): PreviewModel {
  let kind: PreviewModel['kind'] = 'download'
  if (file.mime_type.startsWith('image/')) kind = 'image'
  else if (file.mime_type.startsWith('video/')) kind = 'video'
  else if (file.mime_type.startsWith('audio/')) kind = 'audio'
  else if (file.mime_type === 'application/pdf') kind = 'pdf'
  else if (file.file_name.toLowerCase().endsWith('.docx')) kind = 'docx'
  else if (file.mime_type === 'text/plain') kind = 'text'
  return {kind, url: `/p/${file.public_id}`, name: file.file_name}
}
