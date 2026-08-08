import {useState} from 'react'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'

import {apiClient} from '../../api/client'

interface Token { id: string; scopes: string[]; created_at: string; revoked_at?: string }

export function ApiTokens({plaintext: initialPlaintext}: {plaintext?: string}) {
  const queryClient = useQueryClient()
  const [plaintext, setPlaintext] = useState(initialPlaintext ?? '')
  const [scopes, setScopes] = useState<string[]>(['upload'])
  const tokens = useQuery({queryKey: ['api-tokens'], queryFn: () => apiClient<{items: Token[]}>('/api/v1/tokens'), enabled: initialPlaintext === undefined})
  const create = useMutation({mutationFn: () => apiClient<{plaintext: string}>('/api/v1/tokens', {method: 'POST', body: JSON.stringify({scopes})}), onSuccess: async response => { setPlaintext(response.plaintext); await queryClient.invalidateQueries({queryKey: ['api-tokens']}) }})
  function toggle(scope: string) { setScopes(current => current.includes(scope) ? current.filter(item => item !== scope) : [...current, scope]) }
  function download() {
    const anchor = document.createElement('a')
    anchor.href = URL.createObjectURL(new Blob([plaintext + '\n'], {type: 'text/plain'}))
    anchor.download = 'clist-api-token.txt'
    anchor.click()
    URL.revokeObjectURL(anchor.href)
  }
  return <section className="settings-card"><h2>REST API Token</h2>
    <fieldset><legend>权限</legend>{['upload', 'read', 'manage', 'delete'].map(scope => <label key={scope}><input type="checkbox" checked={scopes.includes(scope)} onChange={() => toggle(scope)} />{scope}</label>)}</fieldset>
    <button type="button" disabled={!scopes.length || create.isPending} onClick={() => create.mutate()}>创建 Token</button>
    {plaintext && <div className="token-once"><strong>Token 明文仅显示一次，请立即保存。</strong><code>{plaintext}</code><button type="button" onClick={() => void navigator.clipboard.writeText(plaintext)}>复制 Token</button><button type="button" onClick={download}>下载 Token</button></div>}
    <ul>{tokens.data?.items.map(token => <li key={token.id}><code>{token.id}</code>：{token.scopes.join(', ')}{token.revoked_at && '（已撤销）'}</li>)}</ul>
  </section>
}
