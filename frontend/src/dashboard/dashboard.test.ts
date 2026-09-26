import { describe, expect, it, vi } from 'vitest'
import { createDashboardWidget, isDashboardConfiguration, launchDashboardWidget, listDashboardWidgets, moveDashboardWidget } from './dashboard'

const placement = { placementId: '11111111-1111-4111-8111-111111111111', pluginId: 'com.example.widget', pluginVersion: '1.0.0', artifactDigest: 'a'.repeat(64), widgetId: 'dashboard', configuration: { title: 'Safe' }, position: 0, enabled: true, revision: 1 }

describe('Dashboard API helpers', () => {
  it('validates authoritative placement data and bounded object configuration', async () => {
    const request = vi.fn().mockResolvedValue({ widgets: [placement] })
    await expect(listDashboardWidgets({ request } as never)).resolves.toEqual({ widgets: [placement] })
    await expect(createDashboardWidget({ pluginId: placement.pluginId, pluginVersion: placement.pluginVersion, artifactDigest: placement.artifactDigest, widgetId: placement.widgetId, configuration: [] as never }, { request } as never)).rejects.toThrow('Invalid Dashboard input')
    expect(isDashboardConfiguration({ value: 'ok' })).toBe(true)
    expect(isDashboardConfiguration('not-an-object')).toBe(false)
  })

  it('sends the complete revision map for reorder and only a placement identity for runtime launch', async () => {
    const request = vi.fn().mockResolvedValueOnce({ widgets: [placement] }).mockResolvedValueOnce({ ...placement, context: { runtimeInstanceId: placement.placementId, pluginId: placement.pluginId, pluginVersion: placement.pluginVersion, artifactDigest: placement.artifactDigest, widgetId: placement.widgetId, widgetType: 'DASHBOARD_WIDGET' }, widgetName: 'Dashboard', runtimeUrl: 'https://plugins.example/runtime', runtimeOrigin: 'https://plugins.example', token: 'token', expiresAt: '2026-09-26T12:00:00Z', capabilities: [] })
    await moveDashboardWidget(placement.placementId, 'up', { [placement.placementId]: 1 }, { request } as never)
    expect(JSON.parse(request.mock.calls[0]![1].body)).toEqual({ expectedRevisions: { [placement.placementId]: 1 } })
    await launchDashboardWidget(placement.placementId, { request } as never)
    expect(request.mock.calls[1]![0]).toBe(`/api/dashboard/widgets/${placement.placementId}/widget-runtime`)
    expect(request.mock.calls[1]![1]).toEqual({ method: 'POST', cache: 'no-store' })
  })
})
