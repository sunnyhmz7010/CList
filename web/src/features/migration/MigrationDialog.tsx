import {useEffect, useState} from 'react'

import {apiClient, type StorageProfile} from '../../api/client'

interface Job { id: string; state: string; progress: number; last_error?: string }

export function MigrationDialog({publicID, profiles, onClose}: {publicID: string; profiles: StorageProfile[]; onClose: () => void}) {
  const [target, setTarget] = useState(profiles[0]?.id ?? '')
  const [job, setJob] = useState<Job>()
  const [error, setError] = useState('')
  useEffect(() => {
    if (!job || ['succeeded', 'failed', 'cleanup_pending'].includes(job.state)) return
    const controller = new AbortController()
    const timer = window.setTimeout(() => void apiClient<Job>(`/api/v1/jobs/${job.id}`, {signal: controller.signal}).then(setJob).catch(() => {}), 1000)
    return () => { controller.abort(); window.clearTimeout(timer) }
  }, [job])
  async function start() {
    try {
      const created = await apiClient<Job>('/api/v1/migrations', {method: 'POST', headers: {'Idempotency-Key': crypto.randomUUID()}, body: JSON.stringify({public_id: publicID, target_profile_id: target})})
      setJob(created)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '迁移失败')
    }
  }
  return <section role="dialog" className="migration-dialog"><h2>迁移文件</h2>
    <label>目标存储<select value={target} onChange={event => setTarget(event.target.value)}>{profiles.map(profile => <option key={profile.id} value={profile.id}>{profile.name}</option>)}</select></label>
    <button type="button" onClick={() => void start()} disabled={!target || Boolean(job)}>开始迁移</button>
    <button type="button" onClick={onClose}>关闭</button>
    {job && <p>状态：{job.state}（{Math.round(job.progress * 100)}%）</p>}
    {job?.last_error && <p role="alert">{job.last_error}，源文件仍保留，可重试。</p>}
    {error && <p role="alert">{error}</p>}
  </section>
}
