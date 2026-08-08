import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {renderToStaticMarkup} from 'react-dom/server'
import {expect, it} from 'vitest'

import {TrashPage} from './TrashPage'

it('requires explicit permanent delete', () => {
  const html = renderToStaticMarkup(<QueryClientProvider client={new QueryClient()}>
    <TrashPage items={[{id: 'b1', name: 'a.txt'}]} />
  </QueryClientProvider>)
  expect(html).toContain('彻底删除')
  expect(html).toContain('彻底删除后无法恢复')
})
