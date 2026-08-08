interface Props {
  jobID: string
  onResolve: (action: 'bind' | 'retry' | 'fail', reference?: string) => void
}

export function UncertainJobDialog({jobID, onResolve}: Props) {
  const bind = () => {
    const reference = window.prompt('请输入频道消息引用')
    if (reference) onResolve('bind', reference)
  }
  return <section role="dialog" aria-labelledby={`uncertain-${jobID}`}>
    <h2 id={`uncertain-${jobID}`}>Telegram 发送结果不确定</h2>
    <p>系统不会自动重试，以免向频道重复发送文件。</p>
    <button type="button" onClick={bind}>绑定频道消息</button>
    <button type="button" onClick={() => onResolve('retry')}>重新上传</button>
    <button type="button" onClick={() => onResolve('fail')}>标记失败</button>
  </section>
}
