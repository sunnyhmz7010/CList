import {TextPreview} from './TextPreview'

export function DocxPreview({url}: {url: string}) {
  return <section><h3>DOCX 只读预览</h3><TextPreview url={url} /></section>
}
