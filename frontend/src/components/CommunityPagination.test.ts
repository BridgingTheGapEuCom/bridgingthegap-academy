import { fireEvent, screen } from '@testing-library/vue'
import { describe, expect, it, vi } from 'vitest'
import { renderWithI18n } from '../test/i18n'
import CommunityPagination from './CommunityPagination.vue'

describe('CommunityPagination', () => {
  it('uses caller copy and exposes bounded navigation', async () => {
    const view = renderWithI18n(CommunityPagination, { props: { limit: 20, offset: 20, total: 41, copy: { label: 'Replies pages', range: (start: number, end: number, total: number) => `${start}-${end}/${total}`, previous: 'Earlier', next: 'Later' }, onPrevious: vi.fn(), onNext: vi.fn() } })
    expect(screen.getByRole('navigation', { name: 'Replies pages' })).toBeTruthy()
    expect(screen.getByText('21-40/41')).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Earlier' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Later' }))
    expect(view.emitted().previous).toHaveLength(1)
    expect(view.emitted().next).toHaveLength(1)
  })
})
