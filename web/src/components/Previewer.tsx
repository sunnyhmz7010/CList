import {DocxPreview} from './previews/DocxPreview'
import {PdfPreview} from './previews/PdfPreview'
import {TextPreview} from './previews/TextPreview'

export interface PreviewModel {
  kind: 'image' | 'video' | 'audio' | 'pdf' | 'docx' | 'text' | 'download'
  url: string
  text?: string
  name?: string
}

export function Previewer({preview}: {preview: PreviewModel}) {
  if (preview.kind === 'image') return <img className="preview-media" src={preview.url} alt={preview.name ?? ''} />
  if (preview.kind === 'video') return <video className="preview-media" controls src={preview.url} />
  if (preview.kind === 'audio') return <audio controls src={preview.url} />
  if (preview.kind === 'pdf') return <PdfPreview url={preview.url} />
  if (preview.kind === 'docx') return <DocxPreview url={preview.url} />
  if (preview.kind === 'text') return <TextPreview url={preview.url} initialText={preview.text} />
  return <a href={preview.url}>下载文件</a>
}
