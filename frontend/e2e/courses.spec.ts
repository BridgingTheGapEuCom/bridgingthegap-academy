import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

const unauthenticated = { type: 'about:blank', title: 'Unauthenticated', status: 401, instance: '/api/auth/session', request_id: 'test-request' }
const courseList = { courses: [{ slug: 'event-driven-architecture', version: { version: '1.10.0', status: 'PUBLISHED', title: 'Event-driven architecture', description: 'A practical course.', source_language: 'en-GB', contributors: [{ display_name: 'Ada Author', role: 'AUTHOR', order: 0 }] } }] }
const courseDetail = {
  course: { slug: 'event-driven-architecture' },
  version: { version: '1.10.0', status: 'PUBLISHED', title: 'Event-driven architecture', description: 'A practical course.', source_language: 'en-GB', objectives: ['Explain event ownership'], contributors: [{ display_name: 'Ada Author', role: 'AUTHOR', order: 0 }], license: { kind: 'STANDARD', identifier: 'CC-BY-4.0', display_name: 'Creative Commons Attribution 4.0', url: 'https://creativecommons.org/licenses/by/4.0/' } },
  modules: [{ module: { key: 'fundamentals', title: 'Fundamentals', description: 'Core concepts.', position: 0 }, lessons: [{ key: 'what-is-eai', title: 'What is EAI?', description: 'A starting point.', objectives: [], estimated_duration_minutes: 10, position: 0, recommended_prerequisite_keys: [] }, { key: 'sync-vs-async', title: 'Synchronous and asynchronous', description: 'Compare approaches.', objectives: [], estimated_duration_minutes: 75, position: 1, recommended_prerequisite_keys: ['what-is-eai'] }] }],
}

const catalogFirst = {
  courseId: '10000000-0000-4000-8000-000000000001', version: '1.10.0', title: 'Event-driven architecture', description: 'A practical published course.', sourceLanguage: 'en-GB',
  license: { kind: 'STANDARD', identifier: 'CC-BY-4.0', displayName: 'Creative Commons Attribution 4.0', url: 'https://creativecommons.org/licenses/by/4.0/', customText: '' },
  contributors: [{ displayName: 'Ada Author', role: 'AUTHOR', order: 0 }], publishedAt: '2026-09-16T12:00:00Z',
}
const catalogSecond = { ...catalogFirst, courseId: '10000000-0000-4000-8000-000000000002', version: '2.0.0', title: 'Boundary design', description: 'The next published catalog page.' }
const publishedCourse = {
  courseId: catalogFirst.courseId, version: catalogFirst.version, title: catalogFirst.title, description: catalogFirst.description,
  objectives: ['Explain event ownership'], sourceLanguage: 'en-GB', changelog: 'First public release.', license: catalogFirst.license,
  contributors: catalogFirst.contributors, publishedAt: catalogFirst.publishedAt,
  modules: [{
    stableKey: 'fundamentals', title: 'Fundamentals', description: 'Core concepts.', position: 0,
    lessons: [
      { stableKey: 'what-is-eai', title: 'What is EAI?', description: 'A starting point.', objectives: ['Recognise integration'], estimatedDurationMinutes: 10, position: 0, prerequisiteStableKeys: [], content: { schemaVersion: 1, blocks: [] } },
      { stableKey: 'sync-vs-async', title: 'Synchronous and asynchronous', description: 'Compare approaches.', objectives: [], estimatedDurationMinutes: 75, position: 1, prerequisiteStableKeys: [], content: { schemaVersion: 1, blocks: [] } },
    ],
  }],
}

async function servePublicCourses(page: Page) {
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 401, contentType: 'application/problem+json', body: JSON.stringify(unauthenticated) }))
  await page.route('**/api/courses', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(courseList) }))
  await page.route('**/api/courses/event-driven-architecture', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(courseDetail) }))
  await page.route(`**/api/courses/by-id/${catalogFirst.courseId}/latest`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(publishedCourse) }))
  await page.route(`**/api/courses/by-id/${catalogFirst.courseId}/versions/${catalogFirst.version}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(publishedCourse) }))
  await page.route('**/api/courses/catalog**', (route) => {
    const offset = new URL(route.request().url()).searchParams.get('offset')
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(offset === '20'
        ? { items: [catalogSecond], limit: 20, offset: 20, total: 21 }
        : { items: [catalogFirst], limit: 20, offset: 0, total: 21 }),
    })
  })
}

test('published course catalog and reader shell are accessible, navigable, and responsive', async ({ page }) => {
  await servePublicCourses(page)
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto('/courses')
  await expect(page.getByRole('heading', { level: 1, name: 'Courses' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Event-driven architecture' })).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByRole('link', { name: 'Event-driven architecture' }).focus()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(/\/courses\/by-id\/10000000-0000-4000-8000-000000000001/)
  await expect(page.getByRole('heading', { level: 1, name: 'Event-driven architecture' })).toBeVisible()
  await expect(page.getByRole('navigation', { name: 'Course lessons' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'What is EAI?, estimated duration 10 min' })).toHaveAttribute('aria-current', 'page')
  await page.getByRole('link', { name: 'Synchronous and asynchronous, estimated duration 1 hr 15 min' }).focus()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(/lesson=sync-vs-async/)
  await expect(page.getByRole('heading', { level: 2, name: 'Synchronous and asynchronous' })).toBeVisible()
  await expect(page.getByRole('heading', { level: 2, name: 'Synchronous and asynchronous' })).toBeFocused()
  await page.goBack()
  await expect(page).toHaveURL(/lesson=what-is-eai/)
  await expect(page.getByRole('link', { name: 'What is EAI?, estimated duration 10 min' })).toHaveAttribute('aria-current', 'page')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByRole('link', { name: 'Browse courses' }).click()
  await page.getByRole('button', { name: 'Next' }).focus()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL('/courses?offset=20')
  await expect(page.getByRole('link', { name: 'Boundary design' })).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  for (const width of [390, 320]) {
    await page.setViewportSize({ width, height: 844 })
    await expect(page.getByRole('heading', { level: 1, name: 'Courses' })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  }
})
