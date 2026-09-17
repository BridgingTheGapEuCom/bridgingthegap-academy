import { describe, expect, it } from 'vitest'
import { loginLocation, safeInternalReturnPath } from './navigation'

describe('authentication navigation', () => {
  it('preserves only safe internal return paths', () => {
    expect(safeInternalReturnPath('/authoring/drafts/a?tab=review')).toBe('/authoring/drafts/a?tab=review')
    expect(safeInternalReturnPath('https://attacker.example')).toBe('/')
    expect(safeInternalReturnPath('//attacker.example')).toBe('/')
    expect(safeInternalReturnPath('/\\attacker.example')).toBe('/')
    expect(loginLocation('/authoring')).toEqual({ path: '/login', query: { returnTo: '/authoring' } })
  })
})
