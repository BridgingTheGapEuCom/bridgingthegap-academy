import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

const draftID = '11111111-1111-4111-8111-111111111111'
const session = { user_id: '22222222-2222-4222-8222-222222222222', expires_at: '2026-09-15T16:00:00Z', csrf_token: 'test-csrf-token' }
const draft = {
  id: draftID,
  course_id: '33333333-3333-4333-8333-333333333333',
  intended_version: '1.0.0',
  source_language: 'en',
  title: 'Integration foundations',
  description: 'A mutable working version.',
  objectives: ['Explain ownership'],
  changelog: 'Initial Draft.',
  license: {
    kind: 'STANDARD',
    identifier: 'CC-BY-4.0',
    display_name: 'Creative Commons Attribution 4.0',
    url: 'https://creativecommons.org/licenses/by/4.0/',
    custom_text: '',
  },
  status: 'ACTIVE',
  revision: 3,
  created_at: '2026-09-15T10:00:00Z',
  updated_at: '2026-09-15T11:00:00Z',
}

const structure = {
  modules: [
    {
      id: '33333333-3333-4333-8333-333333333333',
      stable_key: 'foundations',
      title: 'Foundations',
      description: 'Core concepts.',
      position: 0,
      revision: 2,
      lessons: [{ id: '44444444-4444-4444-8444-444444444444', stable_key: 'what-is-eai', title: 'What is EAI?', description: 'Start here.', objectives: ['Explain EAI'], estimated_duration_minutes: null, position: 0, revision: 2, recommended_prerequisite_keys: [] }],
    },
    { id: '55555555-5555-4555-8555-555555555555', stable_key: 'advanced', title: 'Advanced', description: '', position: 1, revision: 2, lessons: [] },
  ],
}

const lesson = {
  id: '44444444-4444-4444-8444-444444444444',
  draft_id: draftID,
  module_id: structure.modules[0].id,
  stable_key: 'what-is-eai',
  title: 'What is EAI?',
  description: 'Start here.',
  objectives: ['Explain EAI'],
  estimated_duration_minutes: 15,
  position: 0,
  revision: 2,
  recommended_prerequisite_keys: [],
  content: { schemaVersion: 1, blocks: [] },
  created_at: '2026-09-15T10:00:00Z',
  updated_at: '2026-09-15T11:00:00Z',
}

async function serveDraft(page: Page) {
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route(`**/api/authoring/drafts/${draftID}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(draft) }))
  await page.route(`**/api/authoring/drafts/${draftID}/structure`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ modules: [] }) }))
}

test('Authoring Draft shell is private, accessible, and responsive', async ({ page }) => {
  await serveDraft(page)
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto(`/authoring/drafts/${draftID}`)
  await expect(page.getByRole('heading', { level: 1, name: 'Integration foundations' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Overview' })).toHaveAttribute('aria-current', 'page')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByRole('link', { name: 'Structure', exact: true }).focus()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(`/authoring/drafts/${draftID}/structure`)
  await expect(page.getByRole('heading', { level: 2, name: 'Structure' })).toBeVisible()

  await page.setViewportSize({ width: 390, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})

test('Authoring metadata editor saves a changed field with its current revision', async ({ page }) => {
  let patchBody: unknown
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route(`**/api/authoring/drafts/${draftID}`, (route) => {
    if (route.request().method() === 'PATCH') {
      patchBody = route.request().postDataJSON()
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ...draft, title: 'Updated foundations', revision: 4, updated_at: '2026-09-15T12:00:00Z' }),
      })
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(draft) })
  })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/authoring/drafts/${draftID}`)
  const title = page.getByRole('textbox', { name: /^Title\b/ })
  await title.fill('Updated foundations')
  await page.getByRole('button', { name: 'Save changes' }).click()

  await expect(page.getByRole('heading', { level: 1, name: 'Updated foundations' })).toBeVisible()
  expect(patchBody).toEqual({ expectedRevision: 3, title: 'Updated foundations' })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('Authoring structure controls support keyboard reordering without narrow-screen overflow', async ({ page }) => {
  let orderBody: unknown
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route(`**/api/authoring/drafts/${draftID}/structure`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(structure) }))
  await page.route(`**/api/authoring/drafts/${draftID}/modules/order`, (route) => {
    orderBody = route.request().postDataJSON()
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ draftRevision: 4 }) })
  })
  await page.route(`**/api/authoring/drafts/${draftID}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(draft) }))

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/authoring/drafts/${draftID}/structure`)
  await expect(page.getByRole('heading', { level: 3, name: 'Foundations' })).toBeVisible()
  const move = page.getByRole('button', { name: 'Move module down' }).first()
  await move.focus()
  await page.keyboard.press('Enter')
  await expect(page.getByText('Module order updated.')).toBeVisible()
  expect(orderBody).toEqual({ expectedDraftRevision: 3, moduleIds: [structure.modules[1]?.id, structure.modules[0]?.id] })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('Authoring Lesson metadata editor saves with the Lesson revision and remains usable at a narrow width', async ({ page }) => {
  let patchBody: unknown
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route(`**/api/authoring/drafts/${draftID}/structure**`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ modules: [] }) }))
  await page.route(`**/api/authoring/drafts/${draftID}/lessons/${lesson.id}`, (route) => {
    if (route.request().method() === 'PATCH') {
      patchBody = route.request().postDataJSON()
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ...lesson, title: 'Updated Lesson', revision: 3, draftRevision: 4 }) })
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(lesson) })
  })
  await page.route(`**/api/authoring/drafts/${draftID}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(draft) }))

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/authoring/drafts/${draftID}/lessons/${lesson.id}`)
  await expect(page.getByRole('heading', { level: 2, name: 'Lesson metadata' })).toBeVisible()
  await expect(page.getByText('what-is-eai')).toBeVisible()
  const title = page.getByRole('textbox', { name: /^Title\b/ })
  await title.fill('Updated Lesson')
  await page.getByRole('button', { name: 'Save changes' }).click()

  await expect(page.getByText('Lesson metadata saved.')).toBeVisible()
  expect(patchBody).toEqual({ expectedLessonRevision: 2, title: 'Updated Lesson' })
  await page.getByRole('link', { name: 'Back to structure' }).focus()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('Authoring Lesson prerequisites remain advisory, ordered, keyboard-operable, and responsive', async ({ page }) => {
  let prerequisiteBody: unknown
  const prerequisiteStructure = {
    modules: [{
      ...structure.modules[0],
      lessons: [
        structure.modules[0].lessons[0],
        { id: '66666666-6666-4666-8666-666666666666', stable_key: 'intro-to-eai', title: 'Introduction to EAI', description: 'Begin here.', objectives: ['Recognise EAI'], estimated_duration_minutes: 10, position: 1, revision: 2, recommended_prerequisite_keys: [] },
        { id: '77777777-7777-4777-8777-777777777777', stable_key: 'routing', title: 'Message routing', description: 'Route safely.', objectives: ['Route messages'], estimated_duration_minutes: 20, position: 2, revision: 2, recommended_prerequisite_keys: [] },
      ],
    }],
  }
  const lessonWithPrerequisite = { ...lesson, recommended_prerequisite_keys: ['routing'] }
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route(`**/api/authoring/drafts/${draftID}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(draft) }))
  await page.route(`**/api/authoring/drafts/${draftID}/structure**`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(prerequisiteStructure) }))
  await page.route(`**/api/authoring/drafts/${draftID}/lessons/${lesson.id}**`, (route) => {
    if (route.request().method() === 'PUT') {
      prerequisiteBody = route.request().postDataJSON()
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ...lessonWithPrerequisite, revision: 3, draftRevision: 4 }) })
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(lessonWithPrerequisite) })
  })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/authoring/drafts/${draftID}/lessons/${lesson.id}`)
  await expect(page.getByRole('heading', { level: 3, name: 'Recommended prerequisites' })).toBeVisible()
  await expect(page.getByText(/do not restrict access/)).toBeVisible()
  const available = page.getByRole('combobox', { name: 'Available Lessons' })
  await expect(available.locator('option')).toHaveCount(2)
  await available.selectOption('intro-to-eai')
  await page.getByRole('button', { name: 'Add recommended prerequisite' }).click()
  const move = page.getByRole('button', { name: 'Move Introduction to EAI up' })
  await move.focus()
  await page.keyboard.press('Enter')
  await page.getByRole('button', { name: 'Save recommended prerequisites' }).click()

  await expect(page.getByText('Recommended prerequisites saved.')).toBeVisible()
  expect(prerequisiteBody).toEqual({ expectedLessonRevision: 2, prerequisiteLessonKeys: ['intro-to-eai', 'routing'] })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})
