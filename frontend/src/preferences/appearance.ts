import type { components } from '../api/generated'

export type StoredAppearance = components['schemas']['Appearance']
export type ResolvedAppearance = 'light' | 'dark'

export interface ColorSchemeMediaQuery {
  matches: boolean
  addEventListener?(type: 'change', listener: (event: MediaQueryListEvent) => void): void
  removeEventListener?(type: 'change', listener: (event: MediaQueryListEvent) => void): void
}

export function resolveAppearance(preference: StoredAppearance, systemPrefersDark: boolean): ResolvedAppearance {
  if (preference === 'DARK') return 'dark'
  if (preference === 'LIGHT') return 'light'
  return systemPrefersDark ? 'dark' : 'light'
}

export function applyAppearance(appearance: ResolvedAppearance, root: HTMLElement | undefined = typeof document === 'undefined' ? undefined : document.documentElement): void {
  root?.setAttribute('data-theme', appearance)
}

export function colorSchemeMediaQuery(): ColorSchemeMediaQuery | undefined {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return undefined
  return window.matchMedia('(prefers-color-scheme: dark)')
}
