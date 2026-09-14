import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

const unauthenticated = { type: 'about:blank', title: 'Unauthenticated', status: 401, instance: '/api/auth/session', request_id: 'test-request' }
const courseList = { courses: [{ slug: 'event-driven-architecture', version: { version: '1.10.0', status: 'PUBLISHED', title: 'Event-driven architecture', description: 'A practical course.', source_language: 'en-GB', contributors: [{ display_name: 'Ada Author', role: 'AUTHOR', order: 0 }] } }] }
const courseDetail = {
  course: { slug: 'event-driven-architecture' },
  version: { version: '1.10.0', status: 'PUBLISHED', title: 'Event-driven architecture', description: 'A practical course.', source_language: 'en-GB', objectives: ['Explain event ownership'], contributors: [{ display_name: 'Ada Author', role: 'AUTHOR', order: 0 }], license: { kind: 'STANDARD', identifier: 'CC-BY-4.0', display_name: 'Creative Commons Attribution 4.0', url: 'https://creativecommons.org/licenses/by/4.0/' } },
  modules: [{ module: { key: 'fundamentals', title: 'Fundamentals', description: 'Core concepts.', position: 0 }, lessons: [{ key: 'what-is-eai', title: 'What is EAI?', description: 'A starting point.', objectives: [], estimated_duration_minutes: 10, position: 0, recommended_prerequisite_keys: [] }, { key: 'sync-vs-async', title: 'Synchronous and asynchronous', description: 'Compare approaches.', objectives: [], estimated_duration_minutes: 75, position: 1, recommended_prerequisite_keys: ['what-is-eai'] }] }],
}

async function servePublicCourses(page: Page) {
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 401, contentType: 'application/problem+json', body: JSON.stringify(unauthenticated) }))
  await page.route('**/api/courses', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(courseList) }))
  await page.route('**/api/courses/event-driven-architecture', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(courseDetail) }))
}

test('public course discovery and overview are accessible and preserve lesson provenance', async ({ page }) => {
  await servePublicCourses(page)
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto('/courses')
  await expect(page.getByRole('heading', { level: 1, name: 'Courses' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Event-driven architecture' })).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByRole('link', { name: 'Event-driven architecture' }).focus()
  await page.keyboard.press('Enter')
  await expect(page.getByRole('heading', { level: 1, name: 'Event-driven architecture' })).toBeVisible()
  await expect(page.getByText('Recommended before this lesson: What is EAI?')).toBeVisible()
  await expect(page.getByRole('link', { name: 'Synchronous and asynchronous' })).toHaveAttribute('href', '/courses/event-driven-architecture/versions/1.10.0/lessons/sync-vs-async')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  for (const width of [390, 320]) {
    await page.setViewportSize({ width, height: 844 })
    await expect(page.getByRole('heading', { level: 1, name: 'Event-driven architecture' })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  }
})
