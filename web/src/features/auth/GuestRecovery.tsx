import {useState} from 'react'

import {apiClient} from '../../api/client'

export const recoveryStorageKey = 'clist_guest_recovery_key'

export function GuestRecovery({keyValue}: {keyValue?: string}) {
  const [key, setKey] = useState(keyValue ?? '')
  async function recover() {
    await apiClient('/api/v1/vault/recover', {method: 'POST', body: JSON.stringify({key})})
    localStorage.setItem(recoveryStorageKey, key)
  }
  return <section><h2>恢复上传记录</h2>{keyValue && <strong>请立即保存恢复密钥</strong>}
    <textarea value={key} onChange={event => setKey(event.target.value)} aria-label="恢复密钥" />
    <button type="button" onClick={() => void navigator.clipboard.writeText(key)}>复制</button>
    <button type="button" onClick={() => void recover()}>恢复</button>
    <a download="clist-recovery-key.txt" href={`data:text/plain,${encodeURIComponent(key)}`}>下载</a>
  </section>
}
