import {useState} from 'react'
import {apiClient} from '../../api/client'

export function FileAccessDialog({publicId}: {publicId: string}) {
  const [password, setPassword] = useState('')
  return <form onSubmit={event => { event.preventDefault(); void apiClient(`/api/v1/files/${publicId}/password`, {method: 'PUT', body: JSON.stringify({password})}) }}>
    <label>单文件密码<input type="password" value={password} onChange={event => setPassword(event.target.value)} /></label><button>保存</button>
  </form>
}
