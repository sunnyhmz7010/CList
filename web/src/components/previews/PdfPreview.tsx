export function PdfPreview({url}: {url: string}) {
  return <iframe className="preview-frame" src={url} title="PDF 预览" sandbox="allow-same-origin" />
}
