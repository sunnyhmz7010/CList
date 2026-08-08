interface Props {
  publicSecret: string
  headerSecret: string
  onRegenerate: () => void
}

export function WebhookSettings({publicSecret, headerSecret, onRegenerate}: Props) {
  return <fieldset>
    <legend>Telegram Webhook</legend>
    <label>Webhook 路径
      <input readOnly value={`/webhooks/telegram/${publicSecret}`} />
    </label>
    <label>Secret Token
      <input readOnly value={headerSecret} />
    </label>
    <p>密钥只在保存前展示，请复制到 Telegram Webhook 配置。</p>
    <button type="button" onClick={onRegenerate}>重新生成密钥</button>
  </fieldset>
}
