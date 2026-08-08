import {useEffect, useState} from 'react'

export function TextPreview({url, initialText = ''}: {url: string; initialText?: string}) {
  const [text, setText] = useState(initialText)
  useEffect(() => {
    if (initialText) return
    const controller = new AbortController()
    void fetch(url, {credentials: 'same-origin', signal: controller.signal}).then(response => response.ok ? response.text() : Promise.reject(new Error('preview failed'))).then(setText).catch(() => {})
    return () => controller.abort()
  }, [initialText, url])
  return <pre className="preview-text">{text}</pre>
}
