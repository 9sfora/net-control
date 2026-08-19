export type Principal = { sub: string; tenant_id: string; name: string; email: string; permissions: string[]; exp: number }

const TOKEN_KEY = 'netcontrol.access_token'
export const auth = {
  token: () => localStorage.getItem(TOKEN_KEY),
  set: (token: string) => localStorage.setItem(TOKEN_KEY, token),
  clear: () => localStorage.removeItem(TOKEN_KEY),
}

function expireSession() {
  auth.clear()
  window.dispatchEvent(new Event('netcontrol:session-expired'))
}

export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message) }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = auth.token()
  const headers = new Headers(init.headers)
  if (!(init.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const response = await fetch(path, { ...init, headers })
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    if (response.status === 401) expireSession()
    throw new ApiError(response.status, payload?.error?.message || 'Não foi possível concluir a solicitação.')
  }
  return payload as T
}

export async function downloadFile(path: string): Promise<{ blob: Blob; name: string }> {
  const response = await fetch(path, { headers: { Authorization: `Bearer ${auth.token()}` } })
  if (!response.ok) {
    if (response.status === 401) expireSession()
    const payload = await response.json().catch(() => ({}))
    throw new ApiError(response.status, payload?.error?.message || 'Não foi possível baixar o arquivo.')
  }
  const disposition = response.headers.get('Content-Disposition') || ''
  const name = disposition.match(/filename="?([^";]+)"?/i)?.[1] || 'firmware.bin'
  return { blob: await response.blob(), name }
}

export const formatDate = (value?: string | null) => value ? new Intl.DateTimeFormat('pt-BR', { dateStyle: 'short', timeStyle: 'short' }).format(new Date(value)) : '—'
export const label: Record<string, string> = {
  ONLINE: 'Online', STALE: 'Instável', OFFLINE: 'Offline', NEVER_CONNECTED: 'Nunca conectado',
  ACTIVE: 'Ativo', INACTIVE: 'Inativo', BLOCKED: 'Bloqueado', PENDING: 'Pendente',
  WAITING_DEVICE: 'Aguardando', PROCESSING: 'Processando', RETRY_SCHEDULED: 'Nova tentativa',
  COMPLETED: 'Concluído', FAILED: 'Falhou', CANCELED: 'Cancelado', EXPIRED: 'Expirado',
  READY: 'Pronto', NO_FILE: 'Sem arquivo', ACTIVATE: 'Ativação', UPDATE_FIRMWARE: 'Firmware',
}
