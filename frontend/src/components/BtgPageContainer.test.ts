import { cleanup, render } from '@testing-library/vue'
import { afterEach, describe, expect, it } from 'vitest'
import BtgPageContainer from './BtgPageContainer.vue'

afterEach(cleanup)

describe('BtgPageContainer', () => {
  it('uses the standard application page grid by default', () => {
    const { container } = render(BtgPageContainer, { slots: { default: 'Page content' } })
    const page = container.firstElementChild
    expect(page?.classList.contains('btg-page-container--page')).toBe(true)
  })

  it('supports section spacing without changing the canonical page grid', () => {
    const { container } = render(BtgPageContainer, { props: { as: 'section', spacing: 'section' }, slots: { default: 'Section content' } })
    const page = container.firstElementChild
    expect(page?.tagName).toBe('SECTION')
    expect(page?.classList.contains('btg-page-container--section')).toBe(true)
  })
})
