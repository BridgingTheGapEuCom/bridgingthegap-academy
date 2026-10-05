import { fireEvent, render, screen } from '@testing-library/vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'
import AuthoringDirtyActionBar from './AuthoringDirtyActionBar.vue'
import AuthoringDraftHeader from './AuthoringDraftHeader.vue'
import AuthoringLessonHeader from './AuthoringLessonHeader.vue'
import AuthoringSection from './AuthoringSection.vue'

function router(path: string) {
  const instance = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/authoring', component: { template: '<div />' } },
      { path: '/draft/overview', component: { template: '<div />' } },
      { path: '/lesson/details', component: { template: '<div />' } },
      { path: '/structure', component: { template: '<div />' } },
    ],
  })
  void instance.push(path)
  return instance
}

describe('Authoring visual-system components', () => {
  it('renders the canonical Draft header and shared active tab semantics', async () => {
    const instance = router('/draft/overview')
    await instance.isReady()
    render(AuthoringDraftHeader, {
      props: {
        title: 'Integration foundations',
        metadata: [{ label: 'Status', value: 'Active Draft' }],
        tabs: [{ label: 'Overview', to: '/draft/overview' }],
      },
      global: { plugins: [instance] },
    })
    expect(screen.getByRole('heading', { level: 1, name: 'Integration foundations' })).toBeTruthy()
    expect(screen.getByRole('navigation', { name: 'Draft sections' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Overview' }).getAttribute('aria-current')).toBe('page')
    expect(screen.getByText('Active Draft')).toBeTruthy()
  })

  it('renders the lighter Lesson header with the same tab component', async () => {
    const instance = router('/lesson/details')
    await instance.isReady()
    render(AuthoringLessonHeader, {
      props: {
        title: 'What is EAI?',
        backTo: '/structure',
        tabs: [{ label: 'Details', to: '/lesson/details' }],
      },
      global: { plugins: [instance] },
    })
    expect(screen.getByLabelText('Lesson context').textContent).toContain('Lesson')
    expect(screen.getByRole('navigation', { name: 'Lesson sections' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Details' }).getAttribute('aria-current')).toBe('page')
  })

  it('keeps shared sections semantic and exposes the dirty bar only while dirty', async () => {
    render(AuthoringSection, {
      props: { headingId: 'basics-title', title: 'Basic information', description: 'Describe this lesson.' },
      slots: { default: '<label for="title">Title</label><input id="title" />' },
    })
    expect(screen.getByRole('region', { name: 'Basic information' })).toBeTruthy()

    const discard = vi.fn()
    const view = render(AuthoringDirtyActionBar, { props: { show: false, onDiscard: discard } })
    expect(screen.queryByLabelText('Unsaved changes')).toBeNull()
    await view.rerender({ show: true, onDiscard: discard })
    expect(document.querySelector('.authoring-dirty-bar .btg-toolbar')).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Discard changes' }))
    expect(discard).toHaveBeenCalledOnce()
  })
})
