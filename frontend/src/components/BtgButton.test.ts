import { render, screen } from '@testing-library/vue'
import { describe, expect, it } from 'vitest'
import BtgButton from './BtgButton.vue'

describe('BtgButton', () => {
  it('uses a native button with its visible label', () => {
    render(BtgButton, { slots: { default: 'Continue' } })
    expect(screen.getByRole('button', { name: 'Continue' }).tagName).toBe('BUTTON')
  })
})
