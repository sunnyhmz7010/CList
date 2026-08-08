import {renderToStaticMarkup} from 'react-dom/server'
import {expect, it} from 'vitest'

import {CapabilityBadge} from './CapabilityBadge'

it('warns that streaming backend cannot resume', () => {
  const html = renderToStaticMarkup(<CapabilityBadge capabilities={{range: false, head: false, streaming: true}} />)
  expect(html).toContain('当前后端不支持断点续传')
})
