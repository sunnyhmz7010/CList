import type {ReactNode} from 'react'

export function AdminLayout({title, children}: {title: string; children: ReactNode}) {
  return <div className="admin-shell">
    <aside className="admin-sidebar">
      <div className="brand-mark"><span className="brand-icon">C</span><span>CList</span></div>
      <p className="sidebar-caption">个人文件空间</p>
      <nav aria-label="管理导航" className="admin-nav">
        <a className="nav-link" href="/">文件</a>
        <a className="nav-link" href="/gallery">相册</a>
        <a className="nav-link" href="/trash">回收站</a>
        <a className="nav-link nav-link-active" href="/admin">设置</a>
      </nav>
      <div className="sidebar-footer">
        <a className="nav-link" href="/recover">恢复访问</a>
      </div>
    </aside>
    <div className="admin-main">
      <header className="admin-topbar">
        <div>
          <p className="eyebrow">CList 管理后台</p>
          <h1>{title}</h1>
        </div>
        <div className="topbar-actions">
          <label className="search-field"><span className="sr-only">搜索文件</span><input type="search" placeholder="搜索文件" /></label>
          <a className="button button-secondary" href="/admin">设置</a>
        </div>
      </header>
      <main className="admin-content">{children}</main>
    </div>
  </div>
}
