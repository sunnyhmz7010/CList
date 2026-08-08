import type {StorageCapabilities} from '../api/client'

export function CapabilityBadge({capabilities}: {capabilities: StorageCapabilities}) {
  return <span className={capabilities.range ? 'capability capability-ok' : 'capability capability-warning'}>
    {capabilities.range ? '支持断点续传' : '当前后端不支持断点续传'}
    {!capabilities.head && '，不支持 HEAD'}
  </span>
}
