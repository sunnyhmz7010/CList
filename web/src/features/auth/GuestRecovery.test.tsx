import {renderToStaticMarkup} from 'react-dom/server'
import {expect, it} from 'vitest'
import {GuestRecovery} from './GuestRecovery'

it('offers copy and download for a newly created recovery key', () => {
  const html = renderToStaticMarkup(<GuestRecovery keyValue="k" />)
  expect(html).toContain('请立即保存恢复密钥')
  expect(html).toContain('复制')
})
