import { cleanup, fireEvent, render, screen } from '@testing-library/vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import AuthoringInspectorSection from './AuthoringInspectorSection.vue'
import BtgDialog from './BtgDialog.vue'
import BtgDisclosure from './BtgDisclosure.vue'
import BtgDragHandle from './BtgDragHandle.vue'
import BtgMenu from './BtgMenu.vue'
import BtgMetadataStrip from './BtgMetadataStrip.vue'
import BtgOrderedList from './BtgOrderedList.vue'
import BtgOrderedRow from './BtgOrderedRow.vue'
import BtgPreviousNextNavigation from './BtgPreviousNextNavigation.vue'
import BtgSplitWorkspace from './BtgSplitWorkspace.vue'
import BtgToolbar from './BtgToolbar.vue'

afterEach(cleanup)

describe('Academy composition primitives', () => {
  it('keeps toolbar controls in logical leading, content, and action groups', () => {
    render({ components: { BtgToolbar }, template: '<BtgToolbar><template #leading><span>Search</span></template><span>Filters</span><template #actions><button>Save</button></template></BtgToolbar>' })
    const toolbar = document.querySelector('.btg-toolbar')
    expect(toolbar?.querySelector('.btg-toolbar__leading')?.textContent).toContain('Search')
    expect(toolbar?.querySelector('.btg-toolbar__actions')?.textContent).toContain('Save')
  })

  it('renders metadata as a label/value definition list with optional icon', () => {
    render(BtgMetadataStrip, { props: { items: [{ label: 'Module', value: '01 · Foundations', icon: 'module' }, { label: 'Position', value: '2 of 3' }] } })
    expect(screen.getByText('Module').tagName).toBe('DT')
    expect(screen.getByText('01 · Foundations').tagName).toBe('DD')
    expect(document.querySelector('.btg-metadata-strip svg')).toBeTruthy()
  })

  it('opens a dialog from its trigger and returns focus after Escape', async () => {
    render(BtgDialog, { props: { title: 'Move lesson', description: 'Choose a destination.' }, slots: { trigger: '<button type="button">Open move</button>', default: '<p>Dialog body</p>' } })
    const trigger = screen.getByRole('button', { name: 'Open move' })
    await fireEvent.click(trigger)
    expect(screen.getByRole('dialog')).toBeTruthy()
    await fireEvent.keyDown(document, { key: 'Escape' })
    await new Promise((resolve) => setTimeout(resolve))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(document.activeElement).toBe(trigger)
  })

  it('opens the action menu and supports Escape dismissal', async () => {
    render(BtgMenu, { props: { label: 'Lesson actions', items: [{ value: 'move', label: 'Move lesson', icon: 'move' }, { value: 'delete', label: 'Delete lesson', icon: 'delete', danger: true, separatorBefore: true }] } })
    const trigger = screen.getByRole('button', { name: 'Lesson actions' })
    await fireEvent.click(trigger)
    expect(screen.getByRole('menuitem', { name: 'Move lesson' })).toBeTruthy()
    await fireEvent.keyDown(document, { key: 'Escape' })
    await new Promise((resolve) => setTimeout(resolve))
    expect(screen.queryByRole('menu')).toBeNull()
    expect(document.activeElement).toBe(trigger)
  })

  it('binds disclosure state to aria-expanded and controlled content', async () => {
    const Fixture = { components: { BtgDisclosure }, setup: () => ({ open: ref(false) }), template: '<BtgDisclosure v-model="open" label="Module one"><p>Lessons</p></BtgDisclosure>' }
    render(Fixture)
    const trigger = screen.getByRole('button', { name: 'Module one' })
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
    await fireEvent.click(trigger)
    expect(trigger.getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByRole('region', { name: 'Module one' }).textContent).toContain('Lessons')
  })

  it('provides balanced and outline-inspector workspace structures', async () => {
    const { container, rerender } = render(BtgSplitWorkspace, { props: { variant: 'balanced', primaryLabel: 'Outline', secondaryLabel: 'Inspector' }, slots: { primary: 'Left', secondary: 'Right' } })
    expect(container.querySelector('.btg-split-workspace--balanced')).toBeTruthy()
    expect(screen.getByRole('region', { name: 'Outline' }).textContent).toContain('Left')
    await rerender({ variant: 'outline-inspector', primaryLabel: 'Outline', secondaryLabel: 'Inspector' })
    expect(container.querySelector('.btg-split-workspace--outline-inspector')).toBeTruthy()
  })

  it('aligns inspector actions with a labelled heading', () => {
    render(AuthoringInspectorSection, { props: { title: 'Description', icon: 'document' }, slots: { action: '<button>Edit details</button>', default: '<p>Lesson description</p>' } })
    expect(screen.getByRole('heading', { level: 3, name: 'Description' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Edit details' })).toBeTruthy()
  })

  it('renders ordered rows with calm selected state and a named drag handle', async () => {
    const onClick = vi.fn()
    render({ components: { BtgOrderedList, BtgOrderedRow, BtgDragHandle }, setup: () => ({ onClick }), template: '<BtgOrderedList><BtgOrderedRow :index="1" selected>Getting started<template #handle><BtgDragHandle label="Reorder Getting started" @click="onClick" /></template><template #actions><button>More</button></template></BtgOrderedRow></BtgOrderedList>' })
    const row = screen.getByText('Getting started').closest('.btg-ordered-row')
    expect(row?.getAttribute('data-selected')).toBe('true')
    const handle = screen.getByRole('button', { name: 'Reorder Getting started' })
    await fireEvent.click(handle)
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('keeps previous and next controls secondary and named', async () => {
    const previous = vi.fn()
    const next = vi.fn()
    render(BtgPreviousNextNavigation, { props: { navigationLabel: 'Lesson navigation', previousLabel: 'Previous', nextLabel: 'Next', onPrevious: previous, onNext: next } })
    await fireEvent.click(screen.getByRole('button', { name: 'Previous' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(previous).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledOnce()
  })
})
