import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
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

const first = { placementId: '11111111-1111-4111-8111-111111111111', pluginId: 'com.example.widget', pluginVersion: '1.0.0', artifactDigest: 'a'.repeat(64), widgetId: 'first', configuration: { title: 'First' }, position: 0, enabled: true, revision: 1 }
const second = { ...first, placementId: '22222222-2222-4222-8222-222222222222', widgetId: 'second', configuration: { title: 'Second' }, position: 1, enabled: false, revision: 2 }
const available = { pluginId: first.pluginId, pluginName: 'Example', pluginVersion: first.pluginVersion, artifactDigest: first.artifactDigest, widgetId: first.widgetId, widgetName: 'Example Dashboard', description: 'A safe widget' }

function renderPage() {
  return render(DashboardPage, { global: { stubs: { DashboardWidgetPlacementFrame: { props: ['placement'], template: '<div data-testid="runtime">{{ placement.widgetId }}</div>' } } } })
}

describe('DashboardPage', () => {
  beforeEach(() => {
    for (const mock of [listMock, availableMock, createMock, updateMock, moveMock, deleteMock]) mock.mockReset()
    listMock.mockResolvedValue({ widgets: [second, first] })
    availableMock.mockResolvedValue({ widgets: [available] })
  })
  afterEach(cleanup)

  it('uses authoritative placement order and launches enabled placements independently', async () => {
    renderPage()
    const configured = await screen.findByRole('list', { name: 'Configured Dashboard widgets' })
    expect(configured.textContent?.indexOf('first')).toBeLessThan(configured.textContent?.indexOf('second') ?? 0)
    expect(screen.getAllByTestId('runtime')).toHaveLength(1)
    expect(screen.getByTestId('runtime').textContent).toBe('first')
  })

  it('adds an eligible widget and uses only discovery release coordinates', async () => {
    createMock.mockResolvedValue({ ...first, placementId: '33333333-3333-4333-8333-333333333333', position: 2 })
    renderPage(); await screen.findByText('Configure Dashboard widgets')
    await fireEvent.click(screen.getByText('Add dashboard widget'))
    await screen.findByRole('option', { name: /Example Dashboard/ })
    await fireEvent.change(screen.getByRole('combobox'), { target: { value: `${available.pluginId}:${available.pluginVersion}:${available.artifactDigest}:${available.widgetId}` } })
    await fireEvent.click(screen.getByRole('button', { name: 'Add dashboard widget' }))
    await waitFor(() => expect(createMock).toHaveBeenCalledWith(expect.objectContaining({ pluginId: available.pluginId, widgetId: available.widgetId, configuration: {} })))
  })

  it('sends the full current revision map when reordering and replaces state with the response', async () => {
    moveMock.mockResolvedValue({ widgets: [{ ...second, position: 0, revision: 3 }, { ...first, position: 1, revision: 2 }] })
    renderPage(); await screen.findByText('Configure Dashboard widgets')
    await fireEvent.click(screen.getByRole('button', { name: 'Move second up' }))
    await waitFor(() => expect(moveMock).toHaveBeenCalledWith(second.placementId, 'up', { [first.placementId]: 1, [second.placementId]: 2 }))
    expect(screen.getByRole('list', { name: 'Configured Dashboard widgets' }).textContent?.indexOf('second')).toBeLessThan(screen.getByRole('list', { name: 'Configured Dashboard widgets' }).textContent?.indexOf('first') ?? 0)
  })

  it('keeps a conflict explicit and reloads server state instead of overwriting it', async () => {
    updateMock.mockRejectedValue(new APIProblemError(409, undefined, undefined))
    listMock.mockResolvedValueOnce({ widgets: [first, second] }).mockResolvedValueOnce({ widgets: [second, first] })
    renderPage(); await screen.findByText('Configure Dashboard widgets')
    await fireEvent.click(screen.getAllByRole('button', { name: 'Configure placement' })[0]!)
    await fireEvent.click(screen.getByRole('button', { name: 'Save configuration' }))
    await screen.findByRole('alert')
    expect(listMock).toHaveBeenCalledTimes(2)
    expect(screen.getByText(/Dashboard configuration changed/)).toBeTruthy()
  })
})
