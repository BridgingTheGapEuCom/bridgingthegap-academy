import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

const session = { authenticated: true, user_id: '11111111-1111-4111-8111-111111111111', expires_at: '2026-09-26T12:00:00Z', csrf_token: 'test-csrf-token' }
const first = { placementId: '22222222-2222-4222-8222-222222222222', pluginId: 'com.example.dashboard', pluginVersion: '1.0.0', artifactDigest: 'a'.repeat(64), widgetId: 'overview', configuration: { title: 'Overview' }, position: 0, enabled: true, revision: 1 }
const available = { pluginId: first.pluginId, pluginName: 'Example widgets', pluginVersion: first.pluginVersion, artifactDigest: first.artifactDigest, widgetId: first.widgetId, widgetName: 'Overview', description: 'A test Dashboard widget' }

function launch(placement: typeof first) {
  return { context: { runtimeInstanceId: '33333333-3333-4333-8333-333333333333', pluginId: placement.pluginId, pluginVersion: placement.pluginVersion, artifactDigest: placement.artifactDigest, widgetId: placement.widgetId, widgetType: 'DASHBOARD_WIDGET' }, widgetName: 'Overview', runtimeUrl: 'https://plugins.academy.test/runtime#runtime=33333333-3333-4333-8333-333333333333', runtimeOrigin: 'https://plugins.academy.test', token: 'short-lived-token', expiresAt: '2026-09-26T12:05:00Z', capabilities: ['widget.runtime.bootstrap', 'widget.dashboard.context.read'], dashboardContext: { placementId: placement.placementId, configuration: placement.configuration } }
}

async function serveDashboard(page: Page) {
  const placements = [{ ...first }]
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route('**/api/dashboard/widgets/available', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ widgets: [available] }) }))
  await page.route('**/api/dashboard/widgets/*/widget-runtime', (route) => {
    const placement = placements.find((value) => route.request().url().includes(value.placementId))
    return route.fulfill(placement ? { status: 200, contentType: 'application/json', body: JSON.stringify(launch(placement)) } : { status: 404 })
  })
  await page.route('**/api/dashboard/widgets/*/move-*', async (route) => {
    const id = route.request().url().split('/').at(-2)!
    const index = placements.findIndex((value) => value.placementId === id)
    const down = route.request().url().includes('move-down')
    const other = index + (down ? 1 : -1)
    if (other >= 0 && other < placements.length) [placements[index], placements[other]] = [placements[other]!, placements[index]!]
    placements.forEach((placement, position) => { placement.position = position; placement.revision += 1 })
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ widgets: placements }) })
  })
  await page.route('**/api/dashboard/widgets/*', async (route) => {
    const request = route.request(); const id = request.url().split('/').at(-1)!
    if (request.method() === 'PATCH') {
      const body = request.postDataJSON() as { configuration: Record<string, unknown>; enabled: boolean }
      const placement = placements.find((value) => value.placementId === id)!
      placement.configuration = body.configuration; placement.enabled = body.enabled; placement.revision += 1
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(placement) })
    }
    if (request.method() === 'DELETE') {
      const index = placements.findIndex((value) => value.placementId === id); if (index >= 0) placements.splice(index, 1)
      return route.fulfill({ status: 204 })
    }
    return route.fallback()
  })
  await page.route('**/api/dashboard/widgets', async (route) => {
    if (route.request().method() === 'GET') return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ widgets: placements }) })
    if (route.request().method() === 'POST') {
      const body = route.request().postDataJSON() as typeof first
      const placement = { ...body, placementId: '44444444-4444-4444-8444-444444444444', position: placements.length, enabled: true, revision: 1 }
      placements.push(placement)
      return route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(placement) })
    }
    return route.fallback()
  })
  await page.route('https://plugins.academy.test/**', (route) => route.fulfill({ contentType: 'text/html', body: '<script>parent.postMessage({protocol:"btg-widget-runtime",version:1,type:"WIDGET_READY",runtimeInstanceId:location.hash.slice(9),payload:{}},"http://127.0.0.1:4173")</script>' }))
}

test('a manager configures ordered Dashboard widgets without executing plugin code in the Academy page', async ({ page }) => {
  await serveDashboard(page)
  await page.goto('/dashboard')
  await expect(page.getByRole('heading', { name: 'Dashboard', exact: true })).toBeVisible()
  await expect(page.getByTitle('Overview')).toHaveAttribute('sandbox', 'allow-scripts allow-same-origin')
  await page.getByText('Add dashboard widget').click()
  await page.getByRole('combobox').selectOption(`${available.pluginId}:${available.pluginVersion}:${available.artifactDigest}:${available.widgetId}`)
  await page.getByRole('button', { name: 'Add dashboard widget' }).click()
  await expect(page.getByRole('list', { name: 'Configured Dashboard widgets' })).toContainText('overview')
  const firstArticle = page.getByRole('heading', { name: 'overview' }).first().locator('..')
  await firstArticle.getByRole('button', { name: 'Configure placement' }).click()
  await firstArticle.getByLabel(/Configuration/).fill('{"title":"Updated"}')
  await firstArticle.getByRole('button', { name: 'Save configuration' }).click()
  await firstArticle.getByRole('button', { name: 'Disable placement' }).click()
  await expect(page.getByTitle('Overview')).toHaveCount(1)
  await page.setViewportSize({ width: 320, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})
