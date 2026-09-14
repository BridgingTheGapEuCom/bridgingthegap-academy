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

function pendingResponse() {
  let resolve: (response: Response) => void = () => undefined
  const promise = new Promise<Response>((complete) => { resolve = complete })
  return { promise, resolve: (response: Response) => resolve(response) }
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

  it('coalesces concurrent bootstrap calls and ignores a stale bootstrap after login', async () => {
    const bootstrap = pendingResponse()
    const fetcher = vi.fn((path: string) => path === '/api/auth/session'
      ? bootstrap.promise
      : Promise.resolve(jsonResponse(session))) as unknown as typeof fetch
    const service = createAuthService({ fetcher })

    const first = service.bootstrapSession()
    const second = service.bootstrapSession()
    expect(first).toBe(second)
    expect(fetcher).toHaveBeenCalledTimes(1)

    await expect(service.login('admin@example.com', 'test password')).resolves.toMatchObject({ kind: 'authenticated' })
    bootstrap.resolve(jsonResponse(problem(401), 401))
    await first
    expect(service.state.value).toEqual({ status: 'authenticated', userId: session.user_id, expiresAt: session.expires_at })
  })

  it('does not let a delayed session response reinstall an old CSRF token after a newer login', async () => {
    const bootstrap = pendingResponse()
    const newerSession = { ...session, user_id: '9c3dc5dc-ac79-4bdd-8e1a-89a795186ddb', csrf_token: 'new-csrf-token' }
    const fetcher = vi.fn((path: string) => {
      if (path === '/api/auth/session') return bootstrap.promise
      if (path === '/api/auth/login') return Promise.resolve(jsonResponse(newerSession))
      return Promise.resolve(new Response(null, { status: 204 }))
    }) as unknown as typeof fetch
    const service = createAuthService({ fetcher })

    const pendingBootstrap = service.bootstrapSession()
    await service.login('admin@example.com', 'test password')
    bootstrap.resolve(jsonResponse(session))
    await pendingBootstrap
    await service.request('/api/test', { method: 'POST', expectJSON: false })

    expect(service.state.value).toMatchObject({ status: 'authenticated', userId: newerSession.user_id })
    expect(new Headers(requestInit(fetcher, 2).headers).get('X-CSRF-Token')).toBe(newerSession.csrf_token)
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

  it('ignores an old protected-request 401 after a newer login has replaced the session', async () => {
    const oldRequest = pendingResponse()
    const newerSession = { ...session, user_id: '9c3dc5dc-ac79-4bdd-8e1a-89a795186ddb', csrf_token: 'new-csrf-token' }
    const fetcher = vi.fn((path: string) => {
      if (path === '/api/auth/session') return Promise.resolve(jsonResponse(session))
      if (path === '/api/admin/status') return oldRequest.promise
      if (path === '/api/auth/login') return Promise.resolve(jsonResponse(newerSession))
      return Promise.resolve(new Response(null, { status: 204 }))
    }) as unknown as typeof fetch
    const service = createAuthService({ fetcher })
    await service.bootstrapSession()

    const stale = service.request('/api/admin/status')
    await service.login('admin@example.com', 'test password')
    oldRequest.resolve(jsonResponse(problem(401), 401))
    await expect(stale).rejects.toMatchObject({ status: 401 })
    expect(service.state.value).toMatchObject({ status: 'authenticated', userId: newerSession.user_id })
    await service.request('/api/test', { method: 'POST', expectJSON: false })
    expect(new Headers(requestInit(fetcher, 3).headers).get('X-CSRF-Token')).toBe(newerSession.csrf_token)
  })

  it('does not let an in-flight protected response restore state after logout', async () => {
    const oldRequest = pendingResponse()
    const fetcher = vi.fn((path: string) => path === '/api/auth/session'
      ? Promise.resolve(jsonResponse(session))
      : path === '/api/admin/status'
        ? oldRequest.promise
        : Promise.resolve(new Response(null, { status: 204 }))) as unknown as typeof fetch
    const service = createAuthService({ fetcher })
    await service.bootstrapSession()

    const pending = service.request('/api/admin/status')
    await service.logout()
    oldRequest.resolve(jsonResponse({ status: 'ok' }))
    await pending
    expect(service.state.value).toEqual({ status: 'unauthenticated' })
  })

  it('honors a protected 401 during a failing logout for the same session', async () => {
    const protectedRequest = pendingResponse()
    const logoutRequest = pendingResponse()
    const fetcher = vi.fn((path: string) => {
      if (path === '/api/auth/session') return Promise.resolve(jsonResponse(session))
      if (path === '/api/admin/status') return protectedRequest.promise
      return logoutRequest.promise
    }) as unknown as typeof fetch
    const service = createAuthService({ fetcher })
    await service.bootstrapSession()

    const pendingProtected = service.request('/api/admin/status')
    const pendingLogout = service.logout()
    protectedRequest.resolve(jsonResponse(problem(401), 401))
    await expect(pendingProtected).rejects.toMatchObject({ status: 401 })
    logoutRequest.resolve(jsonResponse(problem(500), 500))
    await expect(pendingLogout).resolves.toEqual({ kind: 'unavailable' })
    expect(service.state.value).toEqual({ status: 'unauthenticated' })
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
