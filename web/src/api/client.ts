export interface APIErrorBody {
  code: string
  message: string
  request_id: string
  retriable: boolean
}

export class APIError extends Error {
  constructor(public readonly status: number, public readonly body: APIErrorBody) {
    super(body.message)
  }
}

export async function apiClient<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  if (init.method && !['GET', 'HEAD', 'OPTIONS'].includes(init.method.toUpperCase())) {
    const csrf = readCookie('clist_csrf')
    if (csrf) headers.set('X-CSRF-Token', csrf)
  }
  const response = await fetch(path, {...init, headers, credentials: 'same-origin'})
  if (!response.ok) throw new APIError(response.status, await response.json() as APIErrorBody)
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

function readCookie(name: string) {
  if (typeof document === 'undefined') return ''
  const prefix = `${encodeURIComponent(name)}=`
  return document.cookie.split('; ').find(value => value.startsWith(prefix))?.slice(prefix.length) ?? ''
}
