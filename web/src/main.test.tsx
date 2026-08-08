import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'

import { AuthLanding, Shell } from './app/App'

describe('CList 前端入口', () => {
  it('显示项目名称', () => {
    expect(renderToStaticMarkup(<Shell />)).toContain('CList')
  })

  it('未初始化时显示管理员初始化表单', () => {
    const html = renderToStaticMarkup(<AuthLanding status={{initialized: false, authenticated: false}} />)
    expect(html).toContain('初始化管理员')
    expect(html).toContain('账号')
    expect(html).not.toContain('文件列表加载失败')
  })
})
