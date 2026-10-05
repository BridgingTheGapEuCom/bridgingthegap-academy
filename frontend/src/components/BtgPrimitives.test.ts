import { cleanup, fireEvent, render, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it } from 'vitest'
import { ref } from 'vue'
import BtgBadge from './BtgBadge.vue'
import BtgButton from './BtgButton.vue'
import BtgCheckbox from './BtgCheckbox.vue'
import BtgChip from './BtgChip.vue'
import BtgIcon from './BtgIcon.vue'
import BtgIconButton from './BtgIconButton.vue'
import BtgPanel from './BtgPanel.vue'
import BtgSearchField from './BtgSearchField.vue'
import BtgSection from './BtgSection.vue'
import BtgSelect from './BtgSelect.vue'
import BtgTabs from './BtgTabs.vue'
import BtgTextarea from './BtgTextarea.vue'
import BtgTextInput from './BtgTextInput.vue'

afterEach(cleanup)

describe('Academy core primitives', () => {
  it('keeps icons decorative by default and exposes labelled meaningful icons', () => {
    render(BtgIcon, { props: { name: 'search' } })
    expect(document.querySelector('svg')?.getAttribute('aria-hidden')).toBe('true')
    cleanup()
    render(BtgIcon, { props: { name: 'search', label: 'Search' } })
    expect(screen.getByRole('img', { name: 'Search' })).toBeTruthy()
  })

  it('renders button icons and makes loading buttons unavailable', () => {
    render(BtgButton, { props: { leadingIcon: 'plus', loading: true }, slots: { default: 'Add module' } })
    const button = screen.getByRole('button', { name: 'Add module' }) as HTMLButtonElement
    expect(button.disabled).toBe(true)
    expect(button.getAttribute('aria-busy')).toBe('true')
  })

  it('requires an accessible name for icon-only controls through its label prop', () => {
    render(BtgIconButton, { props: { icon: 'settings', label: 'Open settings' } })
    expect(screen.getByRole('button', { name: 'Open settings' })).toBeTruthy()
  })

  it('updates text, textarea, and select models through native controls', async () => {
    const Fixture = {
      components: { BtgTextInput, BtgTextarea, BtgSelect },
      setup: () => ({ text: ref(''), notes: ref(''), choice: ref('one') }),
      template: '<BtgTextInput v-model="text" aria-label="Text" /><BtgTextarea v-model="notes" aria-label="Notes" /><BtgSelect v-model="choice" aria-label="Choice"><option value="one">One</option><option value="two">Two</option></BtgSelect>',
    }
    render(Fixture)
    await fireEvent.update(screen.getByRole('textbox', { name: 'Text' }), 'Academy')
    await fireEvent.update(screen.getByRole('textbox', { name: 'Notes' }), 'Notes')
    await fireEvent.update(screen.getByRole('combobox', { name: 'Choice' }), 'two')
    expect((screen.getByRole('textbox', { name: 'Text' }) as HTMLInputElement).value).toBe('Academy')
    expect((screen.getByRole('textbox', { name: 'Notes' }) as HTMLTextAreaElement).value).toBe('Notes')
    expect((screen.getByRole('combobox', { name: 'Choice' }) as HTMLSelectElement).value).toBe('two')
  })

  it('exposes invalid and disabled text-control semantics', () => {
    render({ components: { BtgTextInput, BtgTextarea, BtgSelect }, template: '<BtgTextInput aria-label="Title" invalid disabled /><BtgTextarea aria-label="Description" invalid disabled /><BtgSelect aria-label="Language" invalid disabled><option>English</option></BtgSelect>' })
    for (const control of [screen.getByRole('textbox', { name: 'Title' }), screen.getByRole('textbox', { name: 'Description' }), screen.getByRole('combobox', { name: 'Language' })]) {
      expect(control.getAttribute('aria-invalid')).toBe('true')
      expect((control as HTMLInputElement).disabled).toBe(true)
    }
  })

  it('associates checkbox label, helper, error, and indeterminate state', async () => {
    render(BtgCheckbox, { props: { label: 'Publish course', description: 'Makes the course available.', error: 'Resolve the review first.', indeterminate: true } })
    const checkbox = screen.getByRole('checkbox', { name: 'Publish course' }) as HTMLInputElement
    expect(checkbox.indeterminate).toBe(true)
    expect(checkbox.getAttribute('aria-describedby')).toContain('description')
    await fireEvent.click(checkbox)
    expect(checkbox.checked).toBe(true)
  })

  it('provides a labelled search input with a non-interfering decorative icon', async () => {
    render(BtgSearchField, { props: { label: 'Search lessons', placeholder: 'Search lessons…' } })
    const search = screen.getByRole('searchbox', { name: 'Search lessons' }) as HTMLInputElement
    await fireEvent.update(search, 'intro')
    expect(search.value).toBe('intro')
    expect(document.querySelector('.btg-search-field__icon')?.getAttribute('aria-hidden')).toBe('true')
  })

  it('renders compact, non-interactive badge and chip content', () => {
    render({ components: { BtgBadge, BtgChip }, template: '<BtgBadge variant="success" icon="objective">Ready</BtgBadge><BtgChip icon="image">Image</BtgChip>' })
    expect(screen.getByText('Ready').classList.contains('btg-badge')).toBe(true)
    expect(screen.getByText('Image').classList.contains('btg-chip')).toBe(true)
  })

  it('uses roving tab semantics and arrow-key activation', async () => {
    const Fixture = {
      components: { BtgTabs },
      setup: () => ({ selected: ref('overview'), tabs: [{ id: 'overview', label: 'Overview' }, { id: 'content', label: 'Content' }] }),
      template: '<BtgTabs v-model="selected" label="Editor sections" :tabs="tabs" />',
    }
    render(Fixture)
    const overview = screen.getByRole('tab', { name: 'Overview' })
    await fireEvent.keyDown(overview, { key: 'ArrowRight' })
    expect(screen.getByRole('tab', { name: 'Content' }).getAttribute('aria-selected')).toBe('true')
    expect(document.activeElement).toBe(screen.getByRole('tab', { name: 'Content' }))
  })

  it('preserves semantic panel and section structure', () => {
    render({
      components: { BtgPanel, BtgSection, BtgButton },
      template: '<BtgPanel as="article"><BtgSection heading-as="h3"><template #heading>Details</template><template #action><BtgButton>Edit</BtgButton></template><p>Body</p></BtgSection></BtgPanel>',
    })
    expect(screen.getByRole('article')).toBeTruthy()
    expect(screen.getByRole('heading', { level: 3, name: 'Details' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Edit' })).toBeTruthy()
  })
})
