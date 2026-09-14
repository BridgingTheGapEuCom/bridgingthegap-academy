import { APIProblemError, type AdminStatusResponse } from '../api/client'
import { type AuthService, useAuth } from '../auth/auth'

export type AdminAccessOutcome =
  | { kind: 'authorized'; status: AdminStatusResponse['status'] }
  | { kind: 'forbidden' }
  | { kind: 'unauthenticated' }
  | { kind: 'unavailable' }

// This is deliberately a one-request check. It is page state, never a
// frontend authorization cache: the backend remains the capability authority.
export async function checkAdminAccess(service: Pick<AuthService, 'request'> = useAuth()): Promise<AdminAccessOutcome> {
  try {
    const status = await service.request<AdminStatusResponse>('/api/admin/status')
    return { kind: 'authorized', status: status.status }
  } catch (error) {
    if (error instanceof APIProblemError) {
      if (error.status === 401) return { kind: 'unauthenticated' }
      if (error.status === 403) return { kind: 'forbidden' }
    }
    return { kind: 'unavailable' }
  }
}
