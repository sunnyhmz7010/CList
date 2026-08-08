import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {renderToStaticMarkup} from 'react-dom/server'
import {expect, it} from 'vitest'

import {FileList} from './FileList'

it('shows local file list and upload progress', () => {
  const html = renderToStaticMarkup(<QueryClientProvider client={new QueryClient()}>
    <FileList files={[{publicId: 'p1', fileName: 'a.txt', size: 5}]} />
  </QueryClientProvider>)
  expect(html).toContain('a.txt')
  expect(html).toContain('<progress')
})
