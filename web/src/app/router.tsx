import {createBrowserRouter} from 'react-router-dom'

import {FileList} from '../features/files/FileList'
import {FolderTree} from '../features/files/FolderTree'
import {UploadQueue} from '../features/uploads/UploadQueue'

function HomePage() { return <main className="workspace"><FolderTree /><div><UploadQueue /><FileList /></div></main> }
function SetupPage() { return <main><h2>初始化管理员</h2><p>请设置管理员账号和密码。</p></main> }
function AdminPage() { return <main><h2>管理后台</h2><HomePage /></main> }

export function createAppRouter() {
  return createBrowserRouter([
    {path: '/', element: <HomePage />},
    {path: '/setup', element: <SetupPage />},
    {path: '/admin', element: <AdminPage />},
  ])
}
