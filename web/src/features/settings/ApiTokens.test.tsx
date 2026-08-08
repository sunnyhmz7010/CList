import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {renderToStaticMarkup} from 'react-dom/server'
import {expect, it} from 'vitest'

import {ApiTokens} from './ApiTokens'

it('warns that token plaintext is shown once', () => {
  const html = renderToStaticMarkup(<QueryClientProvider client={new QueryClient()}>
    <ApiTokens plaintext="clist_test" />
  </QueryClientProvider>)
  expect(html).toContain('仅显示一次')
  expect(html).toContain('复制 Token')
})
