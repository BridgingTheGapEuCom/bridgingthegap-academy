import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { APIProblemError } from '../api/client'

const listMock = vi.hoisted(() => vi.fn())
const availableMock = vi.hoisted(() => vi.fn())
const createMock = vi.hoisted(() => vi.fn())
const updateMock = vi.hoisted(() => vi.fn())
const moveMock = vi.hoisted(() => vi.fn())
const deleteMock = vi.hoisted(() => vi.fn())
vi.mock('../dashboard/dashboard', async (original) => ({
  ...(await original<typeof import('../dashboard/dashboard')>()),
  listDashboardWidgets: listMock, listAvailableDashboardWidgets: availableMock, createDashboardWidget: createMock,
  updateDashboardWidget: updateMock, moveDashboardWidget: moveMock, deleteDashboardWidget: deleteMock,
}))
import DashboardPage from './DashboardPage.vue'
import { renderWithI18n } from '../test/i18n'
import { pseudoLocalize } from '../i18n/pseudo'

const schema = { fields: [{ key: 'title', type: 'TEXT' as const, label: 'Title', description: 'Shown by the widget.', required: true, maxLength: 80 }] }
const first = { placementId: '11111111-1111-4111-8111-111111111111', pluginId: 'com.example.widget', pluginVersion: '1.0.0', artifactDigest: 'a'.repeat(64), widgetId: 'first', configuration: { title: 'First' }, position: 0, enabled: true, revision: 1 }
const second = { ...first, placementId: '22222222-2222-4222-8222-222222222222', widgetId: 'second', configuration: { title: 'Second' }, position: 1, enabled: false, revision: 2 }
const available = { pluginId: first.pluginId, pluginName: 'Example', pluginVersion: first.pluginVersion, artifactDigest: first.artifactDigest, widgetId: first.widgetId, widgetName: 'Example Dashboard', description: 'A safe widget', configuration: schema }
const noConfiguration = { ...available, widgetId: 'simple', widgetName: 'Simple widget', configuration: null }

function renderPage(locale: 'en' | 'en-XA' = 'en') {
  return renderWithI18n(DashboardPage, { global: { stubs: { DashboardWidgetPlacementFrame: { props: ['placement'], template: '<div data-testid="runtime">{{ placement.widgetId }}</div>' } } } }, locale)
}

async function openManagement() {
  await fireEvent.click(await screen.findByRole('button', { name: 'Manage dashboard' }))
  return screen.findByRole('heading', { name: 'Configure dashboard' })
}

describe('DashboardPage', () => {
  beforeEach(() => {
    for (const mock of [listMock, availableMock, createMock, updateMock, moveMock, deleteMock]) mock.mockReset()
    listMock.mockResolvedValue({ widgets: [second, first] })
    availableMock.mockResolvedValue({ widgets: [available, noConfiguration] })
  })
  afterEach(cleanup)

  it('presents a learner-first Dashboard and launches enabled placements independently', async () => {
    renderPage()
    expect(await screen.findByRole('heading', { level: 1, name: 'Dashboard' })).toBeTruthy()
    expect(screen.queryByRole('list', { name: 'Configured Dashboard widgets' })).toBeNull()
    expect(screen.getAllByTestId('runtime')).toHaveLength(1)
    expect(screen.getByTestId('runtime').textContent).toBe('first')
    expect(await screen.findByRole('button', { name: 'Manage dashboard' })).toBeTruthy()
  })

  it('keeps the learner Dashboard available without management authority', async () => {
    availableMock.mockRejectedValue(new APIProblemError(403, undefined, undefined))
    renderPage()
    expect(await screen.findByTestId('runtime')).toBeTruthy()
    await waitFor(() => expect(availableMock).toHaveBeenCalledOnce())
    expect(screen.queryByRole('button', { name: 'Manage dashboard' })).toBeNull()
  })

  it('renders available widgets as cards and never exposes raw JSON', async () => {
    renderPage()
    await openManagement()
    expect(screen.getAllByRole('heading', { name: 'Example Dashboard' }).length).toBeGreaterThan(0)
    expect(screen.getByRole('heading', { name: 'Simple widget' })).toBeTruthy()
    expect(screen.queryByText(/Configuration \(JSON\)/i)).toBeNull()
    expect(screen.queryByRole('combobox')).toBeNull()

    await fireEvent.click(screen.getByRole('button', { name: 'Add Example Dashboard' }))
    const title = await screen.findByRole('textbox', { name: /Title/ })
    await fireEvent.update(title, 'Recent activity')
    createMock.mockResolvedValue({ ...first, placementId: '33333333-3333-4333-8333-333333333333', configuration: { title: 'Recent activity' }, position: 2 })
    await fireEvent.click(screen.getByRole('button', { name: 'Add widget' }))
    await waitFor(() => expect(createMock).toHaveBeenCalledWith(expect.objectContaining({ pluginId: available.pluginId, widgetId: available.widgetId, configuration: { title: 'Recent activity' } })))
  })

  it('adds a widget with no declared fields directly using an empty object', async () => {
    createMock.mockResolvedValue({ ...first, placementId: '33333333-3333-4333-8333-333333333333', widgetId: 'simple', configuration: {}, position: 2 })
    renderPage()
    await openManagement()
    await fireEvent.click(screen.getByRole('button', { name: 'Add Simple widget' }))
    await waitFor(() => expect(createMock).toHaveBeenCalledWith(expect.objectContaining({ widgetId: 'simple', configuration: {} })))
  })

  it('uses authoritative placement order and sends the complete revision map', async () => {
    moveMock.mockResolvedValue({ widgets: [{ ...second, position: 0, revision: 3 }, { ...first, position: 1, revision: 2 }] })
    renderPage()
    await openManagement()
    const configured = screen.getByRole('list', { name: 'Configured Dashboard widgets' })
    expect(configured.textContent?.indexOf('Example Dashboard')).toBeLessThan(configured.textContent?.indexOf('second') ?? 0)
    await fireEvent.click(screen.getByRole('button', { name: 'Move second up' }))
    await waitFor(() => expect(moveMock).toHaveBeenCalledWith(second.placementId, 'up', { [first.placementId]: 1, [second.placementId]: 2 }))
    expect(within(configured).getAllByRole('listitem')[0]?.textContent).toContain('second')
  })

  it('updates enabled state and requires explicit removal confirmation', async () => {
    updateMock.mockResolvedValue({ ...first, enabled: false, revision: 2 })
    deleteMock.mockResolvedValue(undefined)
    renderPage()
    await openManagement()
    const configured = screen.getByRole('list', { name: 'Configured Dashboard widgets' })
    const firstItem = within(configured).getAllByRole('listitem')[0]!
    await fireEvent.click(within(firstItem).getByRole('button', { name: 'Disable' }))
    await waitFor(() => expect(updateMock).toHaveBeenCalledWith(first.placementId, first.revision, first.configuration, false))
    await fireEvent.click(within(firstItem).getByRole('button', { name: 'Remove' }))
    expect(deleteMock).not.toHaveBeenCalled()
    await fireEvent.click(within(firstItem).getByRole('button', { name: 'Confirm remove' }))
    await waitFor(() => expect(deleteMock).toHaveBeenCalledWith(first.placementId, 2))
  })

  it('keeps form input on a CAS conflict and reloads authoritative state', async () => {
    updateMock.mockRejectedValue(new APIProblemError(409, undefined, undefined))
    listMock.mockResolvedValueOnce({ widgets: [first, second] }).mockResolvedValueOnce({ widgets: [second, first] })
    renderPage()
    await openManagement()
    await fireEvent.click(screen.getByRole('button', { name: 'Configure' }))
    const title = screen.getByRole('textbox', { name: /Title/ })
    await fireEvent.update(title, 'My unsaved title')
    await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    expect((await screen.findByRole('alert')).textContent).toMatch(/Dashboard configuration changed/)
    expect(listMock).toHaveBeenCalledTimes(2)
    expect(screen.getByDisplayValue('My unsaved title')).toBeTruthy()
  })

  it('shows a calm empty picker state', async () => {
    availableMock.mockResolvedValue({ widgets: [] })
    renderPage()
    await openManagement()
    expect(screen.getByText('No dashboard widgets are currently available.')).toBeTruthy()
  })

  it('renders Dashboard controls through the pseudo-locale', async () => {
    renderPage('en-XA')
    expect(await screen.findByRole('heading', { level: 1, name: pseudoLocalize('Dashboard') })).toBeTruthy()
    expect(await screen.findByRole('button', { name: pseudoLocalize('Manage dashboard') })).toBeTruthy()
  })
})
