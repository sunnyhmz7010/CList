import {createBrowserRouter} from 'react-router-dom'

import {FileList} from '../features/files/FileList'
import {FolderTree} from '../features/files/FolderTree'
import {UploadQueue} from '../features/uploads/UploadQueue'
import {GuestRecovery} from '../features/auth/GuestRecovery'
import {AccessSettings} from '../features/settings/AccessSettings'
import {StorageProfiles} from '../features/settings/StorageProfiles'

function HomePage() { return <main className="workspace"><FolderTree /><div><UploadQueue /><FileList /></div></main> }
function SetupPage() { return <main><h2>初始化管理员</h2><p>请设置管理员账号和密码。</p></main> }
function AdminPage() { return <main><h1>管理后台</h1><AccessSettings /><StorageProfiles /><HomePage /></main> }

export function createAppRouter() {
  return createBrowserRouter([
    {path: '/', element: <HomePage />},
    {path: '/setup', element: <SetupPage />},
    {path: '/admin', element: <AdminPage />},
    {path: '/recover', element: <GuestRecovery />},
  ])
}
