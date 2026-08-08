import {renderToStaticMarkup} from 'react-dom/server'
import {expect, it} from 'vitest'

import {Previewer} from './Previewer'

it.each([
  ['image', '<img'], ['video', '<video'], ['audio', '<audio'], ['text', '<pre'],
] as const)('renders %s preview', (kind, tag) => {
  expect(renderToStaticMarkup(<Previewer preview={{kind, url: '/p/1'}} />)).toContain(tag)
})
