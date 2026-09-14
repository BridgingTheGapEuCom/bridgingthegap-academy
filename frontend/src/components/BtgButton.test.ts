import { cleanup, render, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import BtgButton from './BtgButton.vue'

describe('BtgButton', () => {
  afterEach(cleanup)

  it('uses a native button with its visible label', () => {
    render(BtgButton, { slots: { default: 'Continue' } })
    expect(screen.getByRole('button', { name: 'Continue' }).tagName).toBe('BUTTON')
  })

  it('does not activate when disabled', async () => {
    const onClick = vi.fn()
    const { getByRole } = render(BtgButton, { props: { disabled: true }, slots: { default: 'Continue' } })
    const button = getByRole('button', { name: 'Continue' })
    button.addEventListener('click', onClick)
    expect((button as HTMLButtonElement).disabled).toBe(true)
    ;(button as HTMLButtonElement).click()
    expect(onClick).not.toHaveBeenCalled()
  })
})
