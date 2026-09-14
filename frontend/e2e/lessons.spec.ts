import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

const unauthenticated = { type: 'about:blank', title: 'Unauthenticated', status: 401, instance: '/api/auth/session', request_id: 'test-request' }
const lessonPath = '/courses/event-driven-architecture/versions/1.10.0/lessons/sync-vs-async'
const lesson = {
  course: { slug: 'event-driven-architecture' },
  version: { version: '1.10.0', status: 'PUBLISHED', title: 'Event-driven architecture' },
  module: { key: 'fundamentals', title: 'Fundamentals', position: 0 },
  lesson: {
    key: 'sync-vs-async', title: 'Synchronous and asynchronous', description: 'Compare approaches.', objectives: ['Explain the trade-off'], estimated_duration_minutes: 10, position: 1, recommended_prerequisite_keys: [],
    content: {
      schemaVersion: 1,
      blocks: [
        { key: 'intro', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'A calm introduction.', marks: [{ type: 'strong' }] }] }] } } },
        { key: 'heading', type: 'HEADING', payload: { level: 2, content: [{ type: 'text', text: 'Examples', marks: [] }] } },
        { key: 'image', type: 'IMAGE', payload: { asset: { assetKey: 'event-flow' }, decorative: false, altText: 'Event flow diagram', caption: 'Event flow' } },
        { key: 'video', type: 'VIDEO', payload: { asset: { assetKey: 'walkthrough' }, title: 'Walkthrough', transcript: 'Video transcript', captionsAsset: { assetKey: 'captions' } } },
        { key: 'audio', type: 'AUDIO', payload: { asset: { assetKey: 'audio-guide' }, title: 'Audio guide', transcript: 'Audio transcript' } },
        { key: 'code', type: 'CODE', payload: { code: 'publish(event)', language: 'text', title: 'Example code' } },
        { key: 'quote', type: 'QUOTE', payload: { text: 'Events create useful boundaries.', attribution: 'Ada Author' } },
        { key: 'callout', type: 'CALLOUT', payload: { kind: 'TIP', title: 'Keep it small', content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Start with one event.', marks: [] }] }] } } },
        { key: 'table', type: 'TABLE', payload: { caption: 'Comparison', headers: ['Style', 'Flow'], rows: [['Sync', 'Waits'], ['Async', 'Continues']] } },
        { key: 'download', type: 'DOWNLOAD', payload: { asset: { assetKey: 'worksheet' }, label: 'Worksheet' } },
        { key: 'check', type: 'KNOWLEDGE_CHECK', payload: { assessmentKey: 'sync-check' } },
        { key: 'divider', type: 'DIVIDER', payload: {} },
      ],
    },
  },
}
const structure = { course: lesson.course, version: lesson.version, modules: [{ module: lesson.module, lessons: [lesson.lesson] }] }

async function serveLesson(page: Page) {
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 401, contentType: 'application/problem+json', body: JSON.stringify(unauthenticated) }))
  await page.route('**/api/courses/event-driven-architecture/versions/1.10.0/lessons/sync-vs-async', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(lesson) }))
  await page.route('**/api/courses/event-driven-architecture/versions/1.10.0', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(structure) }))
}

test('mixed canonical lesson content renders safely in continuous and Focus modes', async ({ page }) => {
  await serveLesson(page)
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto(lessonPath)
  await expect(page.getByRole('heading', { level: 1, name: 'Synchronous and asynchronous' })).toBeVisible()
  await expect(page.getByRole('heading', { level: 2, name: 'Examples' })).toBeVisible()
  await expect(page.getByRole('img', { name: 'Event flow diagram' })).toBeVisible()
  await expect(page.getByRole('table', { name: 'Comparison' })).toBeVisible()
  await expect(page.getByText('Interactive knowledge checks will be available when assessments are enabled.')).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByLabel('Focus').focus()
  await page.keyboard.press('Space')
  await expect(page.getByText('Block 1 of 12')).toBeVisible()
  await expect(page.getByText('A calm introduction.')).toBeVisible()
  await expect(page.getByText('publish(event)')).not.toBeVisible()
  await page.getByRole('button', { name: 'Next block' }).focus()
  await page.keyboard.press('Enter')
  await expect(page.getByText('Block 2 of 12')).toBeVisible()
  await expect(page.getByRole('heading', { level: 2, name: 'Examples' })).toBeVisible()
  await expect(page.getByText('A calm introduction.')).not.toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('lesson reflows at narrow widths without page-wide horizontal overflow', async ({ page }) => {
  await serveLesson(page)
  for (const width of [768, 390, 320]) {
    await page.setViewportSize({ width, height: 844 })
    await page.goto(lessonPath)
    await expect(page.getByRole('heading', { level: 1, name: 'Synchronous and asynchronous' })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  }
})
