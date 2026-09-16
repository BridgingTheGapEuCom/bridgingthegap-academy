import type { components, paths } from './generated'

export type LiveResponse = paths['/health/live']['get']['responses'][200]['content']['application/json']
export type LoginRequest = components['schemas']['LoginRequest']
export type AuthenticatedSessionResponse = components['schemas']['AuthenticatedSession']
export type AdminStatusResponse = components['schemas']['AdminStatus']
export type ProblemDetails = components['schemas']['Problem']
// Some endpoints extend the shared Problem Details envelope with structured
// domain data. Keep that data intact for the feature that owns its display.
export type APIProblemDetails = ProblemDetails
  | components['schemas']['PublicationConflictProblem']
  | components['schemas']['PublicationValidationProblem']

type Fetcher = typeof fetch
type UnsafeMethod = 'POST' | 'PUT' | 'PATCH' | 'DELETE'

export type APIRequestOptions = Omit<RequestInit, 'body' | 'credentials' | 'headers' | 'method'> & {
  body?: BodyInit | null
  csrf?: boolean
  expectJSON?: boolean
  headers?: HeadersInit
  invalidateOnUnauthorized?: boolean
  method?: string
}

export class APIProblemError extends Error {
  constructor(
    readonly status: number,
    readonly problem: APIProblemDetails | undefined,
    readonly retryAfterSeconds: number | undefined,
  ) {
    super(problem?.title ?? `Request failed with status ${status}`)
    this.name = 'APIProblemError'
  }
}

export class APIUnavailableError extends Error {
  constructor(cause?: unknown) {
    super('The application is unavailable')
    this.name = 'APIUnavailableError'
    if (cause !== undefined) this.cause = cause
  }
}

export interface APIClient {
  request<T>(path: string, options?: APIRequestOptions): Promise<T>
}

export interface APIClientOptions {
  fetcher?: Fetcher
  getCSRFToken?: () => string | undefined
  onAuthenticatedSessionUnauthorized?: () => void
}

export function createAPIClient(options: APIClientOptions = {}): APIClient {
  const fetcher = options.fetcher ?? fetch

  return {
    async request<T>(path: string, requestOptions: APIRequestOptions = {}): Promise<T> {
      const {
        body,
        csrf,
        expectJSON = true,
        headers: requestHeaders,
        invalidateOnUnauthorized = true,
        method = 'GET',
        ...fetchOptions
      } = requestOptions
      const normalizedMethod = method.toUpperCase()
      const headers = new Headers(requestHeaders)
      const shouldAttachCSRF = csrf ?? isUnsafeMethod(normalizedMethod)
      const csrfToken = shouldAttachCSRF ? options.getCSRFToken?.() : undefined
      if (csrfToken) headers.set('X-CSRF-Token', csrfToken)

      let response: Response
      try {
        response = await fetcher(path, {
          ...fetchOptions,
          body,
          credentials: 'same-origin',
          headers,
          method: normalizedMethod,
        })
      } catch (error) {
        throw new APIUnavailableError(error)
      }

      if (!response.ok) {
        const problem = await parseProblemDetails(response)
        const problemError = new APIProblemError(response.status, problem, parseRetryAfter(response.headers.get('Retry-After')))
        if (response.status === 401 && invalidateOnUnauthorized) options.onAuthenticatedSessionUnauthorized?.()
        throw problemError
      }

      if (!expectJSON || response.status === 204) return undefined as T
      try {
        return await response.json() as T
      } catch (error) {
        throw new APIUnavailableError(error)
      }
    },
  }
}

export const apiClient = createAPIClient()

export async function getLiveness(): Promise<LiveResponse> {
  return apiClient.request<LiveResponse>('/health/live')
}

function isUnsafeMethod(method: string): method is UnsafeMethod {
  return method === 'POST' || method === 'PUT' || method === 'PATCH' || method === 'DELETE'
}

function parseRetryAfter(value: string | null): number | undefined {
  if (!value || !/^\d+$/.test(value)) return undefined
  const seconds = Number(value)
  return Number.isSafeInteger(seconds) ? seconds : undefined
}

async function parseProblemDetails(response: Response): Promise<APIProblemDetails | undefined> {
  try {
    const value: unknown = await response.json()
    return isProblemDetails(value) ? value : undefined
  } catch {
    return undefined
  }
}

function isProblemDetails(value: unknown): value is ProblemDetails {
  if (typeof value !== 'object' || value === null) return false
  const candidate = value as Record<string, unknown>
  return typeof candidate.type === 'string'
    && typeof candidate.title === 'string'
    && typeof candidate.status === 'number'
    && typeof candidate.instance === 'string'
    && typeof candidate.request_id === 'string'
}
