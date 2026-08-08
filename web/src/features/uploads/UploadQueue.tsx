import {useState} from 'react'
import {useQueryClient} from '@tanstack/react-query'

import {apiClient} from '../../api/client'
import {recoveryStorageKey} from '../auth/GuestRecovery'

const chunkSize = 8 * 1024 * 1024
interface UploadState { id: string; missing_chunks: number[] }

export function UploadQueue() {
  const [progress, setProgress] = useState(0)
  const [message, setMessage] = useState('')
  const queryClient = useQueryClient()

  async function upload(file: File) {
    if (!localStorage.getItem(recoveryStorageKey)) {
      const created = await apiClient<{recovery_key: string}>('/api/v1/vault', {method: 'POST'})
      localStorage.setItem(recoveryStorageKey, created.recovery_key)
    }
    setMessage('正在准备上传')
    const totalChunks = Math.max(1, Math.ceil(file.size / chunkSize))
    const upload = await apiClient<UploadState>('/api/v1/uploads', {
      method: 'POST', headers: {'Idempotency-Key': crypto.randomUUID()},
      body: JSON.stringify({file_name: file.name, mime_type: file.type || 'application/octet-stream', total_size: file.size,
        chunk_size: chunkSize, total_chunks: totalChunks, sha256: await sha256(file)}),
    })
    const state = await apiClient<UploadState>(`/api/v1/uploads/${upload.id}`)
    const missing = [...state.missing_chunks]
    let completed = totalChunks - missing.length
    for (let offset = 0; offset < missing.length; offset += 3) {
      await Promise.all(missing.slice(offset, offset + 3).map(async index => {
        const part = file.slice(index * chunkSize, Math.min(file.size, (index + 1) * chunkSize))
        await apiClient<void>(`/api/v1/uploads/${upload.id}/chunks/${index}`, {
          method: 'PUT', headers: {'X-Chunk-SHA256': await sha256(part)}, body: part,
        })
        completed += 1
        setProgress(Math.round(completed / totalChunks * 100))
      }))
    }
    await apiClient(`/api/v1/uploads/${upload.id}/complete`, {method: 'POST', headers: {'Idempotency-Key': crypto.randomUUID()}})
    setMessage('上传完成')
    await queryClient.invalidateQueries({queryKey: ['files']})
  }

  return <section><h2>上传</h2>
    <input type="file" multiple onChange={event => { for (const file of Array.from(event.target.files ?? [])) void upload(file) }} />
    <progress max="100" value={progress} aria-valuenow={progress} /><p aria-live="polite">{message}</p>
  </section>
}

async function sha256(blob: Blob) {
  const digest = await crypto.subtle.digest('SHA-256', await blob.arrayBuffer())
  return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('')
}
