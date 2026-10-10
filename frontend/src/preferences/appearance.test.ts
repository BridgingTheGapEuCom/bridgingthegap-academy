import { afterEach, describe, expect, it } from 'vitest'
import { applyAppearance, resolveAppearance } from './appearance'

afterEach(() => document.documentElement.removeAttribute('data-theme'))

describe('application appearance', () => {
  it('resolves SYSTEM from the browser and explicit choices independently', () => {
    expect(resolveAppearance('SYSTEM', false)).toBe('light')
    expect(resolveAppearance('SYSTEM', true)).toBe('dark')
    expect(resolveAppearance('LIGHT', true)).toBe('light')
    expect(resolveAppearance('DARK', false)).toBe('dark')
  })

  it('applies the resolved value at the document root', () => {
    applyAppearance('dark')
    expect(document.documentElement.dataset.theme).toBe('dark')
  })
})
