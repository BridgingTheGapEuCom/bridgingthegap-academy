import { describe, expect, it } from 'vitest'
import { canonicalLanguageTag, filterLanguageOptions, languageLabel } from './languages'

describe('Academy language presentation', () => {
  it('canonicalizes valid tags for picker input', () => {
    expect(canonicalLanguageTag(' EN ')).toBe('en')
    expect(canonicalLanguageTag('pt-br')).toBe('pt-BR')
    expect(canonicalLanguageTag('zh-hans')).toBe('zh-Hans')
    expect(canonicalLanguageTag('English')).toBeUndefined()
  })

  it('searches the discovery set by label and tag', () => {
    expect(filterLanguageOptions('spanish').map((option) => option.tag)).toContain('es')
    expect(filterLanguageOptions('es-ES').map((option) => option.tag)).toContain('es-ES')
  })

  it('uses a readable label while retaining the tag', () => {
    expect(languageLabel('en-GB')).toContain('en-GB')
    expect(languageLabel('pt-BR')).toContain('pt-BR')
  })
})
