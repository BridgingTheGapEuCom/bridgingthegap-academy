import { describe, expect, it, vi } from 'vitest'
import { APIProblemError, createAPIClient } from './client'

const problem = (status: number) => ({
  type: 'about:blank',
  title: 'Request failed',
  status,
  instance: '/api/test',
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

function headersFor(fetcher: ReturnType<typeof responseFetcher>, call = 0): Headers {
  return new Headers((fetcher as unknown as ReturnType<typeof vi.fn>).mock.calls[call][1].headers)
}

describe('API client', () => {
  it('uses same-origin credentials and leaves GET requests free of CSRF headers', async () => {
    const fetcher = responseFetcher(jsonResponse({ status: 'ok' }))
    const client = createAPIClient({ fetcher, getCSRFToken: () => 'csrf-secret' })

    await client.request('/health/live')

    expect(fetcher).toHaveBeenCalledWith('/health/live', expect.objectContaining({ credentials: 'same-origin', method: 'GET' }))
    expect(headersFor(fetcher).get('X-CSRF-Token')).toBeNull()
  })

  it('attaches the in-memory CSRF token to unsafe methods only', async () => {
    const fetcher = responseFetcher(
      new Response(null, { status: 204 }),
      new Response(null, { status: 204 }),
      new Response(null, { status: 204 }),
      new Response(null, { status: 204 }),
    )
    const client = createAPIClient({ fetcher, getCSRFToken: () => 'csrf-secret' })

    for (const method of ['POST', 'PUT', 'PATCH', 'DELETE']) {
      await client.request('/api/test', { method, expectJSON: false })
    }

    for (let index = 0; index < 4; index += 1) expect(headersFor(fetcher, index).get('X-CSRF-Token')).toBe('csrf-secret')
  })

  it('parses Problem Details and invalidates only authenticated-session 401 responses', async () => {
    const onUnauthorized = vi.fn()
    const client = createAPIClient({ fetcher: responseFetcher(jsonResponse(problem(401), 401)), onAuthenticatedSessionUnauthorized: onUnauthorized })

    await expect(client.request('/api/protected')).rejects.toMatchObject<Partial<APIProblemError>>({ status: 401, problem: problem(401) })
    expect(onUnauthorized).toHaveBeenCalledOnce()
  })

  it('keeps 403 and 429 available to callers without invalidating authentication', async () => {
    const onUnauthorized = vi.fn()
    const client = createAPIClient({
      fetcher: responseFetcher(jsonResponse(problem(403), 403), jsonResponse(problem(429), 429, { 'Retry-After': '12' })),
      onAuthenticatedSessionUnauthorized: onUnauthorized,
    })

    await expect(client.request('/api/protected')).rejects.toMatchObject<Partial<APIProblemError>>({ status: 403 })
    await expect(client.request('/api/login', { method: 'POST', csrf: false, invalidateOnUnauthorized: false })).rejects.toMatchObject<Partial<APIProblemError>>({ status: 429, retryAfterSeconds: 12 })
    expect(onUnauthorized).not.toHaveBeenCalled()
  })
})
