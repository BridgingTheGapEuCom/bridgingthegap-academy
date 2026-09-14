import { describe, expect, it, vi } from 'vitest'
import { auth, createAuthService, useAuth } from './auth'

const session = {
  authenticated: true as const,
  user_id: '3c5e54ca-37fb-4af0-938d-694a1815d1ed',
  expires_at: '2026-09-15T12:00:00Z',
  csrf_token: 'csrf-token-for-current-session',
}

const problem = (status: number) => ({
  type: 'about:blank',
  title: 'Request failed',
  status,
  instance: '/api/auth/session',
  request_id: 'request-123',
})

function jsonResponse(value: unknown, status = 200, headers: HeadersInit = {}): Response {
  return new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json', ...headers } })
}

function responseFetcher(...responses: Array<Response | Error>) {
  const fetcher = vi.fn(async () => {
    const response = responses.shift()
    if (response instanceof Error) throw response
    if (!response) throw new Error('Missing mocked response')
    return response
  }) as unknown as typeof fetch
  return fetcher
}

function requestInit(fetcher: ReturnType<typeof responseFetcher>, call = 0): RequestInit {
  return (fetcher as unknown as ReturnType<typeof vi.fn>).mock.calls[call][1] as RequestInit
}

describe('authentication state', () => {
  it('uses one intentional module-level state instance', () => {
    expect(useAuth()).toBe(auth)
    expect(auth.state.value).toEqual({ status: 'bootstrapping' })
  })

  it('bootstraps an authenticated server session without exposing its CSRF token in state', async () => {
    const service = createAuthService({ fetcher: responseFetcher(jsonResponse(session)) })

    await service.bootstrapSession()

    expect(service.state.value).toEqual({ status: 'authenticated', userId: session.user_id, expiresAt: session.expires_at })
    expect(JSON.stringify(service.state.value)).not.toContain(session.csrf_token)
  })

  it('treats bootstrap 401 as unauthenticated and operational failures as unavailable', async () => {
    const unauthenticated = createAuthService({ fetcher: responseFetcher(jsonResponse(problem(401), 401)) })
    const unavailable = createAuthService({ fetcher: responseFetcher(jsonResponse(problem(500), 500)) })
    const networkFailure = createAuthService({ fetcher: responseFetcher(new Error('network unavailable')) })

    await unauthenticated.bootstrapSession()
    await unavailable.bootstrapSession()
    await networkFailure.bootstrapSession()

    expect(unauthenticated.state.value).toEqual({ status: 'unauthenticated' })
    expect(unavailable.state.value).toEqual({ status: 'unavailable' })
    expect(networkFailure.state.value).toEqual({ status: 'unavailable' })
  })
})

describe('authentication service operations', () => {
  it('uses the generated login request shape, updates state, and keeps the password out of global state', async () => {
    const fetcher = responseFetcher(jsonResponse(problem(401), 401), jsonResponse(session))
    const service = createAuthService({ fetcher })
    await service.bootstrapSession()

    const result = await service.login('learner@example.com', 'correct horse battery staple')

    expect(result).toEqual({ kind: 'authenticated', userId: session.user_id, expiresAt: session.expires_at })
    expect((fetcher as unknown as ReturnType<typeof vi.fn>).mock.calls[1][0]).toBe('/api/auth/login')
    expect(JSON.parse(String(requestInit(fetcher, 1).body))).toEqual({ email: 'learner@example.com', password: 'correct horse battery staple' })
    expect(new Headers(requestInit(fetcher, 1).headers).get('X-CSRF-Token')).toBeNull()
    expect(JSON.stringify(service.state.value)).not.toContain('correct horse battery staple')
  })

  it('keeps invalid credentials, rate limiting, and operational login failures distinct', async () => {
    const invalidCredentials = createAuthService({ fetcher: responseFetcher(jsonResponse(problem(401), 401)) })
    const rateLimited = createAuthService({ fetcher: responseFetcher(jsonResponse(problem(429), 429, { 'Retry-After': '9' })) })
    const unavailable = createAuthService({ fetcher: responseFetcher(jsonResponse(problem(500), 500)) })

    await expect(invalidCredentials.login('learner@example.com', 'password')).resolves.toEqual({ kind: 'invalid-credentials' })
    await expect(rateLimited.login('learner@example.com', 'password')).resolves.toEqual({ kind: 'rate-limited', retryAfterSeconds: 9 })
    await expect(unavailable.login('learner@example.com', 'password')).resolves.toEqual({ kind: 'unavailable' })
  })

  it('attaches CSRF on logout and clears it only after the backend confirms the session is gone', async () => {
    const fetcher = responseFetcher(jsonResponse(session), new Response(null, { status: 204 }), new Response(null, { status: 204 }))
    const service = createAuthService({ fetcher })
    await service.bootstrapSession()

    await expect(service.logout()).resolves.toEqual({ kind: 'unauthenticated' })
    expect(service.state.value).toEqual({ status: 'unauthenticated' })
    expect(new Headers(requestInit(fetcher, 1).headers).get('X-CSRF-Token')).toBe(session.csrf_token)

    await service.logout()
    expect(new Headers(requestInit(fetcher, 2).headers).get('X-CSRF-Token')).toBeNull()
  })

  it('handles an already-gone session safely while preserving authenticated state for forbidden and unavailable logout failures', async () => {
    const alreadyGone = createAuthService({ fetcher: responseFetcher(jsonResponse(session), jsonResponse(problem(401), 401)) })
    await alreadyGone.bootstrapSession()
    await expect(alreadyGone.logout()).resolves.toEqual({ kind: 'unauthenticated' })
    expect(alreadyGone.state.value).toEqual({ status: 'unauthenticated' })

    const forbidden = createAuthService({ fetcher: responseFetcher(jsonResponse(session), jsonResponse(problem(403), 403)) })
    await forbidden.bootstrapSession()
    await expect(forbidden.logout()).resolves.toEqual({ kind: 'forbidden' })
    expect(forbidden.state.value).toEqual({ status: 'authenticated', userId: session.user_id, expiresAt: session.expires_at })

    const unavailable = createAuthService({ fetcher: responseFetcher(jsonResponse(session), new Error('network unavailable')) })
    await unavailable.bootstrapSession()
    await expect(unavailable.logout()).resolves.toEqual({ kind: 'unavailable' })
    expect(unavailable.state.value).toEqual({ status: 'authenticated', userId: session.user_id, expiresAt: session.expires_at })
  })

  it('clears authenticated state and the memory-only CSRF token after a protected request receives 401', async () => {
    const fetcher = responseFetcher(jsonResponse(session), jsonResponse(problem(401), 401), new Response(null, { status: 204 }))
    const service = createAuthService({ fetcher })
    await service.bootstrapSession()

    await expect(service.request('/api/admin/status')).rejects.toMatchObject({ status: 401 })
    expect(service.state.value).toEqual({ status: 'unauthenticated' })

    await service.logout()
    expect(new Headers(requestInit(fetcher, 2).headers).get('X-CSRF-Token')).toBeNull()
  })

  it('keeps authenticated state on 403 and 429 protected-request responses', async () => {
    const fetcher = responseFetcher(jsonResponse(session), jsonResponse(problem(403), 403), jsonResponse(problem(429), 429))
    const service = createAuthService({ fetcher })
    await service.bootstrapSession()

    await expect(service.request('/api/admin/status')).rejects.toMatchObject({ status: 403 })
    await expect(service.request('/api/admin/status')).rejects.toMatchObject({ status: 429 })
    expect(service.state.value).toEqual({ status: 'authenticated', userId: session.user_id, expiresAt: session.expires_at })
  })

  it('does not persist authentication or CSRF state in browser storage', async () => {
    const getItem = vi.spyOn(Storage.prototype, 'getItem')
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    const removeItem = vi.spyOn(Storage.prototype, 'removeItem')
    const service = createAuthService({ fetcher: responseFetcher(jsonResponse(session), new Response(null, { status: 204 })) })

    try {
      await service.bootstrapSession()
      await service.logout()
      expect(getItem).not.toHaveBeenCalled()
      expect(setItem).not.toHaveBeenCalled()
      expect(removeItem).not.toHaveBeenCalled()
    } finally {
      getItem.mockRestore()
      setItem.mockRestore()
      removeItem.mockRestore()
    }
  })
})
