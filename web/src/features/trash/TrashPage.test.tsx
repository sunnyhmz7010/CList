import {renderToStaticMarkup} from 'react-dom/server'
import {expect, it} from 'vitest'

import {TrashPage} from './TrashPage'

it('requires explicit permanent delete', () => {
  const html = renderToStaticMarkup(<TrashPage items={[{id: 'b1', name: 'a.txt'}]} />)
  expect(html).toContain('彻底删除')
  expect(html).toContain('彻底删除后无法恢复')
})
