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

  const transitionToUnauthenticated = () => {
    csrfToken = undefined
    state.value = { status: 'unauthenticated' }
  }
  const transport = createAPIClient({
    fetcher: options.fetcher,
    getCSRFToken: () => csrfToken,
    onAuthenticatedSessionUnauthorized: transitionToUnauthenticated,
  })

  const applyAuthenticatedSession = (session: AuthenticatedSessionResponse) => {
    csrfToken = session.csrf_token
    state.value = { status: 'authenticated', userId: session.user_id, expiresAt: session.expires_at }
  }

  return {
    state: readonly(state),

    async bootstrapSession(): Promise<AuthenticationState> {
      state.value = { status: 'bootstrapping' }
      try {
        const session = await transport.request<AuthenticatedSessionResponse>('/api/auth/session', {
          invalidateOnUnauthorized: false,
        })
        applyAuthenticatedSession(session)
      } catch (error) {
        if (isStatus(error, 401)) transitionToUnauthenticated()
        else state.value = { status: 'unavailable' }
      }
      return state.value
    },

    async login(email: LoginRequest['email'], password: LoginRequest['password']): Promise<LoginOutcome> {
      try {
        const session = await transport.request<AuthenticatedSessionResponse>('/api/auth/login', {
          method: 'POST',
          body: JSON.stringify({ email, password } satisfies LoginRequest),
          headers: { 'Content-Type': 'application/json' },
          csrf: false,
          invalidateOnUnauthorized: false,
        })
        applyAuthenticatedSession(session)
        return { kind: 'authenticated', userId: session.user_id, expiresAt: session.expires_at }
      } catch (error) {
        if (isStatus(error, 401)) return { kind: 'invalid-credentials' }
        if (isStatus(error, 429)) return { kind: 'rate-limited', retryAfterSeconds: retryAfter(error) }
        return { kind: 'unavailable' }
      }
    },

    async logout(): Promise<LogoutOutcome> {
      try {
        await transport.request<void>('/api/auth/logout', { method: 'POST', expectJSON: false, invalidateOnUnauthorized: false })
        transitionToUnauthenticated()
        return { kind: 'unauthenticated' }
      } catch (error) {
        if (isStatus(error, 401)) {
          transitionToUnauthenticated()
          return { kind: 'unauthenticated' }
        }
        if (isStatus(error, 403)) return { kind: 'forbidden' }
        return { kind: 'unavailable' }
      }
    },

    request<T>(path: string, requestOptions?: APIRequestOptions): Promise<T> {
      return transport.request<T>(path, requestOptions)
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
