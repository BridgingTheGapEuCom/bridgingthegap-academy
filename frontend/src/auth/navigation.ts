import type { RouteLocationNormalizedLoaded, RouteLocationRaw } from 'vue-router'

// Return paths are navigation hints only. They remain inside this SPA and are
// never persisted or treated as a URL supplied by a third party.
export function safeInternalReturnPath(value: unknown, fallback = '/'): string {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//') || value.includes('://') || value.includes('\\')) return fallback
  return value
}

export function loginLocation(returnTo: string): RouteLocationRaw {
  const path = safeInternalReturnPath(returnTo)
  return path === '/' ? { path: '/login' } : { path: '/login', query: { returnTo: path } }
}

export function routeReturnPath(route: Pick<RouteLocationNormalizedLoaded, 'fullPath'>): string {
  return safeInternalReturnPath(route.fullPath)
}
