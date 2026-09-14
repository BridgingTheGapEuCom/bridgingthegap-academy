import { describe, expect, it, vi } from 'vitest'
import { APIProblemError, APIUnavailableError } from '../api/client'
import { checkAdminAccess } from './adminAccess'

const problem = (status: number) => new APIProblemError(status, {
  type: 'about:blank',
  title: 'Request failed',
  status,
  instance: '/api/admin/status',
  request_id: 'request-123',
}, undefined)

describe('checkAdminAccess', () => {
  it('uses the protected backend endpoint rather than frontend role state', async () => {
    const request = vi.fn().mockResolvedValue({ status: 'ok' })

    await expect(checkAdminAccess({ request })).resolves.toEqual({ kind: 'authorized', status: 'ok' })
    expect(request).toHaveBeenCalledWith('/api/admin/status')
  })

  it.each([
    [401, { kind: 'unauthenticated' }],
    [403, { kind: 'forbidden' }],
  ] as const)('maps HTTP %s without exposing authorization internals', async (status, expected) => {
    await expect(checkAdminAccess({ request: vi.fn().mockRejectedValue(problem(status)) })).resolves.toEqual(expected)
  })

  it('fails closed when the protected endpoint is unavailable', async () => {
    await expect(checkAdminAccess({ request: vi.fn().mockRejectedValue(new APIUnavailableError()) })).resolves.toEqual({ kind: 'unavailable' })
  })
})
