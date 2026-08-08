import {useState} from 'react'
import {apiClient} from '../../api/client'

export function AccessSettings() {
  const [home, setHome] = useState(''); const [gallery, setGallery] = useState('')
  const save = (scope: string, password: string) => apiClient(`/api/v1/admin/access/${scope}`, {method: 'PUT', body: JSON.stringify({password})})
  return <section className="settings-card"><h2>访问设置</h2>
    <label>首页密码<input type="password" value={home} onChange={e => setHome(e.target.value)} /></label><button onClick={() => void save('home', home)}>保存</button>
    <label>相册密码<input type="password" value={gallery} onChange={e => setGallery(e.target.value)} /></label><button onClick={() => void save('gallery', gallery)}>保存</button>
  </section>
}
