import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'

import { Shell } from './app/App'

describe('CList 前端入口', () => {
  it('显示项目名称', () => {
    expect(renderToStaticMarkup(<Shell />)).toContain('CList')
  })
})
