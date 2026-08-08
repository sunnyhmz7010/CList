import {createBrowserRouter} from 'react-router-dom'

import {FileList} from '../features/files/FileList'
import {FolderTree} from '../features/files/FolderTree'
import {UploadQueue} from '../features/uploads/UploadQueue'
import {GuestRecovery} from '../features/auth/GuestRecovery'
import {AdminGate} from '../features/auth/AdminGate'
import {AccessSettings} from '../features/settings/AccessSettings'
import {StorageProfiles} from '../features/settings/StorageProfiles'
import {Gallery} from '../features/gallery/Gallery'
import {GuestGate} from '../features/auth/GuestGate'
import {TrashPage} from '../features/trash/TrashPage'
import {ApiTokens} from '../features/settings/ApiTokens'
import {Diagnostics} from '../features/settings/Diagnostics'
import {AdminLayout} from './AdminLayout'

function HomePage() { return <main className="workspace"><FolderTree /><div><UploadQueue /><FileList /></div></main> }
function SetupPage() { return <main><h2>初始化管理员</h2><p>请设置管理员账号和密码。</p></main> }
function AdminPage() { return <div className="settings-grid"><AccessSettings /><StorageProfiles /><ApiTokens /><Diagnostics /></div> }

export function createAppRouter() {
  return createBrowserRouter([
    {path: '/', element: <AdminGate><AdminLayout title="文件管理"><HomePage /></AdminLayout></AdminGate>},
    {path: '/setup', element: <SetupPage />},
    {path: '/admin', element: <AdminGate><AdminLayout title="设置"><AdminPage /></AdminLayout></AdminGate>},
    {path: '/recover', element: <GuestRecovery />},
    {path: '/gallery', element: <GuestGate scope="gallery"><Gallery /></GuestGate>},
    {path: '/trash', element: <AdminGate><AdminLayout title="回收站"><TrashPage /></AdminLayout></AdminGate>},
  ])
}
