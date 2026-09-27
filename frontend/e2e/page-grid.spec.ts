import { expect, test, type Page } from '@playwright/test'

const session = {
  authenticated: true,
  user_id: '22222222-2222-4222-8222-222222222222',
  expires_at: '2027-01-01T00:00:00Z',
  csrf_token: 'test-csrf-token',
}

type RouteUnderTest = {
  path: string
  container: string
  heading: string
}

const routes: RouteUnderTest[] = [
  { path: '/', container: '.home-page__hero.btg-page-container', heading: '#home-title' },
  { path: '/dashboard', container: '.dashboard-page.btg-page-container', heading: '#dashboard-title' },
  { path: '/courses', container: '.courses-page.btg-page-container', heading: '#courses-title' },
  { path: '/authoring', container: '.authoring-home.btg-page-container', heading: '#authoring-home-title' },
]

type Bounds = { left: number; right: number; width: number }

async function servePageGrid(page: Page) {
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route('**/api/courses/catalog**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [], limit: 20, offset: 0, total: 0 }) }))
  await page.route('**/api/authoring/drafts', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ drafts: [] }) }))
  await page.route('**/api/dashboard/widgets/available', (route) => route.fulfill({ status: 403, contentType: 'application/problem+json', body: JSON.stringify({ title: 'Forbidden', status: 403 }) }))
  await page.route('**/api/dashboard/widgets', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ widgets: [] }) }))
}

async function bounds(page: Page, route: RouteUnderTest): Promise<Bounds> {
  await page.goto(route.path)
  await expect(page.locator(route.heading)).toBeVisible()
  return page.locator(route.container).evaluate((element) => {
    const box = element.getBoundingClientRect()
    return { left: box.left, right: box.right, width: box.width }
  })
}

function expectSameCanvas(actual: Bounds, expected: Bounds) {
  expect(Math.abs(actual.left - expected.left)).toBeLessThanOrEqual(1)
  expect(Math.abs(actual.right - expected.right)).toBeLessThanOrEqual(1)
  expect(Math.abs(actual.width - expected.width)).toBeLessThanOrEqual(1)
}

test('normal Academy routes share one rendered page canvas at every responsive gutter', async ({ page }) => {
  await servePageGrid(page)

  for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 900 }, { width: 768, height: 900 }, { width: 320, height: 844 }]) {
    await page.setViewportSize(viewport)
    const reference = await bounds(page, routes[0]!)
    const header = await page.locator('.site-header__inner').evaluate((element) => {
      const box = element.getBoundingClientRect()
      return { left: box.left, right: box.right, width: box.width }
    })
    expectSameCanvas(header, reference)
    for (const route of routes.slice(1)) expectSameCanvas(await bounds(page, route), reference)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  }
})

test('Home keeps its full-width course band while its content shares the Dashboard canvas', async ({ page }) => {
  await servePageGrid(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  const home = await bounds(page, routes[0]!)
  const band = await page.locator('.home-page__course-band').evaluate((element) => element.getBoundingClientRect().width)
  expect(band).toBeGreaterThan(home.width)
  expectSameCanvas(await page.locator('.home-page__course-band .btg-page-container').evaluate((element) => {
    const box = element.getBoundingClientRect()
    return { left: box.left, right: box.right, width: box.width }
  }), home)
  expectSameCanvas(await bounds(page, routes[1]!), home)
})

test('the canonical page grid remains aligned and free of horizontal overflow at 200% text', async ({ page }) => {
  await servePageGrid(page)
  await page.setViewportSize({ width: 320, height: 844 })
  let reference: Bounds | undefined
  for (const route of routes) {
    await page.goto(route.path)
    await expect(page.locator(route.heading)).toBeVisible()
    await page.locator('html').evaluate((element) => { element.style.fontSize = '200%' })
    const current = await page.locator(route.container).evaluate((element) => {
      const box = element.getBoundingClientRect()
      return { left: box.left, right: box.right, width: box.width }
    })
    if (reference) expectSameCanvas(current, reference)
    else reference = current
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  }
})
