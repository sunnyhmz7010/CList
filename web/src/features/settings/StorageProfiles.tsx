import {useMemo, useState} from 'react'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'

import {apiClient, type StorageProfile} from '../../api/client'
import {CapabilityBadge} from '../../components/CapabilityBadge'
import {WebhookSettings} from './WebhookSettings'

function secret() {
  return crypto.randomUUID().replaceAll('-', '')
}

export function StorageProfiles() {
  const queryClient = useQueryClient()
  const profiles = useQuery({queryKey: ['storage-profiles'], queryFn: () => apiClient<{items: StorageProfile[]}>('/api/v1/storage-profiles')})
  const [type, setType] = useState<StorageProfile['type']>('telegram_official')
  const [name, setName] = useState('Telegram')
  const [baseURL, setBaseURL] = useState('https://api.telegram.org')
  const [botToken, setBotToken] = useState('')
  const [channelID, setChannelID] = useState('')
  const [publicSecret, setPublicSecret] = useState(() => secret())
  const [headerSecret, setHeaderSecret] = useState(() => secret())
  const isTelegram = type !== 'local'
  const capabilityPreview = useMemo(() => type === 'telegram_streaming'
    ? {range: false, head: false, streaming: true}
    : {range: true, head: true, streaming: true}, [type])
  const create = useMutation({
    mutationFn: () => apiClient<StorageProfile>('/api/v1/storage-profiles', {method: 'POST', body: JSON.stringify({
      type, name, enabled: true, config: isTelegram ? {
        base_url: baseURL, bot_token: botToken, channel_id: channelID,
        webhook_public_secret: publicSecret, webhook_secret: headerSecret,
      } : {root: baseURL},
    })}),
    onSuccess: async () => {
      setBotToken('')
      setPublicSecret(secret())
      setHeaderSecret(secret())
      await queryClient.invalidateQueries({queryKey: ['storage-profiles']})
    },
  })

  return <section className="settings-card">
    <h2>存储档案</h2>
    <ul className="profile-list">{profiles.data?.items.map(profile => <li key={profile.id}>
      <span>{profile.name}（{profile.type}）</span><CapabilityBadge capabilities={profile.capabilities} />
    </li>)}</ul>
    <form onSubmit={event => { event.preventDefault(); create.mutate() }}>
      <label>类型<select value={type} onChange={event => setType(event.target.value as StorageProfile['type'])}>
        <option value="telegram_official">官方 Telegram Bot API</option>
        <option value="telegram_streaming">自建 Streaming API</option>
        <option value="local">Local</option>
      </select></label>
      <label>名称<input value={name} onChange={event => setName(event.target.value)} required /></label>
      <label>{isTelegram ? 'API 基础地址' : '容器内绝对路径'}<input value={baseURL} onChange={event => setBaseURL(event.target.value)} required /></label>
      {isTelegram && <>
        <label>Bot Token<input type="password" autoComplete="off" value={botToken} onChange={event => setBotToken(event.target.value)} required /></label>
        <label>频道 ID<input value={channelID} onChange={event => setChannelID(event.target.value)} required /></label>
        <CapabilityBadge capabilities={capabilityPreview} />
        <WebhookSettings publicSecret={publicSecret} headerSecret={headerSecret} onRegenerate={() => { setPublicSecret(secret()); setHeaderSecret(secret()) }} />
      </>}
      <button type="submit" disabled={create.isPending}>保存档案</button>
      {create.error && <p role="alert">保存失败：{create.error.message}</p>}
    </form>
  </section>
}
