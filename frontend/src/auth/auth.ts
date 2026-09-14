import { readonly, ref, type Ref } from 'vue'
import {
  APIProblemError,
  createAPIClient,
  type APIRequestOptions,
  type AuthenticatedSessionResponse,
  type LoginRequest,
} from '../api/client'

export type AuthenticationState =
  | { status: 'bootstrapping' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; userId: string; expiresAt: string }
  | { status: 'unavailable' }

export type LoginOutcome =
  | { kind: 'authenticated'; userId: string; expiresAt: string }
  | { kind: 'invalid-credentials' }
  | { kind: 'rate-limited'; retryAfterSeconds?: number }
  | { kind: 'unavailable' }

export type LogoutOutcome =
  | { kind: 'unauthenticated' }
  | { kind: 'forbidden' }
  | { kind: 'unavailable' }

export interface AuthService {
  readonly state: Readonly<Ref<AuthenticationState>>
  bootstrapSession(): Promise<AuthenticationState>
  login(email: LoginRequest['email'], password: LoginRequest['password']): Promise<LoginOutcome>
  logout(): Promise<LogoutOutcome>
  request<T>(path: string, options?: APIRequestOptions): Promise<T>
}

export interface AuthServiceOptions {
  fetcher?: typeof fetch
}

export function createAuthService(options: AuthServiceOptions = {}): AuthService {
  const state = ref<AuthenticationState>({ status: 'bootstrapping' })
  let csrfToken: string | undefined
  // Operation order prevents an older bootstrap/login/logout result from
  // replacing a newer decision. Session order protects the CSRF token and
  // lets an in-flight request's 401 apply only to the session that sent it.
  let operationVersion = 0
  let sessionVersion = 0
  let logoutInFlight = 0
  let bootstrapInFlight: Promise<AuthenticationState> | undefined

  const transitionToUnauthenticated = () => {
    operationVersion += 1
    sessionVersion += 1
    csrfToken = undefined
    state.value = { status: 'unauthenticated' }
  }
  const transport = createAPIClient({
    fetcher: options.fetcher,
    getCSRFToken: () => csrfToken,
  })

  const applyAuthenticatedSession = (session: AuthenticatedSessionResponse) => {
    operationVersion += 1
    sessionVersion += 1
    csrfToken = session.csrf_token
    state.value = { status: 'authenticated', userId: session.user_id, expiresAt: session.expires_at }
  }

  return {
    state: readonly(state),

    bootstrapSession(): Promise<AuthenticationState> {
      if (bootstrapInFlight) return bootstrapInFlight
      const version = ++operationVersion
      sessionVersion += 1
      state.value = { status: 'bootstrapping' }
      csrfToken = undefined
      const task = (async () => {
        try {
          const session = await transport.request<AuthenticatedSessionResponse>('/api/auth/session', {
            invalidateOnUnauthorized: false,
          })
          if (version === operationVersion) applyAuthenticatedSession(session)
        } catch (error) {
          if (version === operationVersion) {
            if (isStatus(error, 401)) transitionToUnauthenticated()
            else state.value = { status: 'unavailable' }
          }
        }
        return state.value
      })()
      bootstrapInFlight = task
      void task.then(() => { if (bootstrapInFlight === task) bootstrapInFlight = undefined })
      return task
    },

    async login(email: LoginRequest['email'], password: LoginRequest['password']): Promise<LoginOutcome> {
      const version = ++operationVersion
      try {
        const session = await transport.request<AuthenticatedSessionResponse>('/api/auth/login', {
          method: 'POST',
          body: JSON.stringify({ email, password } satisfies LoginRequest),
          headers: { 'Content-Type': 'application/json' },
          csrf: false,
          invalidateOnUnauthorized: false,
        })
        if (version !== operationVersion) return { kind: 'unavailable' }
        applyAuthenticatedSession(session)
        return { kind: 'authenticated', userId: session.user_id, expiresAt: session.expires_at }
      } catch (error) {
        if (version !== operationVersion) return { kind: 'unavailable' }
        if (state.value.status === 'bootstrapping') state.value = { status: 'unavailable' }
        if (isStatus(error, 401)) return { kind: 'invalid-credentials' }
        if (isStatus(error, 429)) return { kind: 'rate-limited', retryAfterSeconds: retryAfter(error) }
        return { kind: 'unavailable' }
      }
    },

    async logout(): Promise<LogoutOutcome> {
      const version = ++operationVersion
      logoutInFlight += 1
      try {
        await transport.request<void>('/api/auth/logout', { method: 'POST', expectJSON: false, invalidateOnUnauthorized: false })
        if (version !== operationVersion) return { kind: 'unavailable' }
        transitionToUnauthenticated()
        return { kind: 'unauthenticated' }
      } catch (error) {
        if (version !== operationVersion) return { kind: 'unavailable' }
        if (isStatus(error, 401)) {
          transitionToUnauthenticated()
          return { kind: 'unauthenticated' }
        }
        if (isStatus(error, 403)) return { kind: 'forbidden' }
        return { kind: 'unavailable' }
      } finally {
        logoutInFlight -= 1
      }
    },

    async request<T>(path: string, requestOptions?: APIRequestOptions): Promise<T> {
      const version = operationVersion
      const sessionAtDispatch = sessionVersion
      const authenticated = state.value.status === 'authenticated'
      try {
        return await transport.request<T>(path, { ...requestOptions, invalidateOnUnauthorized: false })
      } catch (error) {
        if (authenticated && state.value.status === 'authenticated' && sessionAtDispatch === sessionVersion
          && (version === operationVersion || logoutInFlight > 0)
          && requestOptions?.invalidateOnUnauthorized !== false && isStatus(error, 401)) {
          transitionToUnauthenticated()
        }
        throw error
      }
    },
  }
}

function isStatus(error: unknown, status: number): boolean {
  return error instanceof APIProblemError && error.status === status
}

function retryAfter(error: unknown): number | undefined {
  return error instanceof APIProblemError ? error.retryAfterSeconds : undefined
}

// This is the sole application instance. Components may consume it through
// useAuth(), but must never create a new authentication state of their own.
export const auth = createAuthService()

export function useAuth(): AuthService {
  return auth
}
