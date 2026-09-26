import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

const session = { authenticated: true, user_id: '11111111-1111-4111-8111-111111111111', expires_at: '2026-09-26T12:00:00Z', csrf_token: 'test-csrf-token' }
const release = { pluginId: 'com.example.widget', version: '1.0.0', artifactDigest: 'a'.repeat(64), name: 'Example widget', description: 'A deterministic package fixture', publisherName: 'Example', entrypoints: [{ id: 'dashboard', type: 'DASHBOARD_WIDGET', name: 'Dashboard widget' }], currentTrust: 'UNKNOWN', validationStatus: 'VALIDATED_AT_REGISTRATION', signatureStatus: 'UNSIGNED', approvalState: 'NONE', enabled: false, executionPermitted: false, installedAt: '2026-09-26T12:00:00Z' }
const key = { keyId: 'approval-1', purpose: 'BTG_APPROVAL_SIGNING', allowedPluginIds: [], enabled: true, fingerprint: 'sha256:abc' }

async function servePlugins(page: Page) {
  const releases = [{ ...release }]; const keys = [{ ...key }]
  await page.route('**/api/auth/session', (route) => route.fulfill({ contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route('**/api/plugins/**', async (route) => {
    const url = route.request().url()
    if (url.includes('/keys/')) { const keyID = url.split('/').at(-2)!; const found = keys.find((value) => value.keyId === keyID)!; found.enabled = url.endsWith('/enable'); return route.fulfill({ contentType: 'application/json', body: JSON.stringify(found) }) }
    if (url.endsWith('/keys')) { if (route.request().method() === 'GET') return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ keys }) }); const body = route.request().postDataJSON() as typeof key; keys.push({ ...body, enabled: true, fingerprint: 'sha256:new' }); return route.fulfill({ contentType: 'application/json', body: JSON.stringify(keys.at(-1)) }) }
    if (url.endsWith('/approval')) { releases[0]!.approvalState = route.request().method() === 'DELETE' ? 'REVOKED' : 'ACTIVE'; return route.fulfill({ contentType: 'application/json', body: JSON.stringify(releases[0]) }) }
    if (url.endsWith('/enable')) { releases[0]!.enabled = true; return route.fulfill({ contentType: 'application/json', body: JSON.stringify(releases[0]) }) }
    if (url.endsWith('/disable')) { releases[0]!.enabled = false; return route.fulfill({ contentType: 'application/json', body: JSON.stringify(releases[0]) }) }
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify(releases[0]) })
  })
  await page.route('**/api/plugins', async (route) => { if (route.request().method() === 'GET') return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ plugins: releases }) }); return route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(releases[0]) }) })
}

test('an administrator manages plugin releases and public verification keys accessibly', async ({ page }) => {
  await servePlugins(page); await page.goto('/admin/plugins')
  await expect(page.getByRole('heading', { name: 'Plugin management' })).toBeVisible()
  await expect(page.getByText('This is a valid installed release that is not currently BTG-owned or BTG-approved.')).toBeVisible()
  await page.getByLabel(/Key ID/).fill('owned-1'); await page.getByLabel(/Public key/).fill('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'); await page.getByLabel(/Allowed plugin IDs/).fill('com.example.widget')
  await page.getByRole('button', { name: 'Add verification key' }).click()
  await expect(page.getByText('owned-1')).toBeVisible()
  await page.setViewportSize({ width: 320, height: 844 })
  await page.locator('html').evaluate((element) => { element.style.fontSize = '200%' })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})
