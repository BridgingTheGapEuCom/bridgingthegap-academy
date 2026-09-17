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

const reviewCycle = {
  id: '99999999-9999-4999-8999-999999999999',
  draftId: draftID,
  draftRevision: 3,
  snapshotSchemaVersion: 1,
  status: 'IN_REVIEW',
  reviewRevision: 1,
  submittedBy: '66666666-6666-4666-8666-666666666666',
  submittedAt: '2026-09-15T12:00:00Z',
  decidedBy: null,
  decidedAt: null,
}

const approvedReviewCycle = {
  ...reviewCycle,
  status: 'APPROVED',
  reviewRevision: 2,
  decidedBy: session.user_id,
  decidedAt: '2026-09-15T13:00:00Z',
}

const reviewSnapshot = {
  schemaVersion: 1,
  draft: {
    id: draftID, revision: 3, courseId: draft.course_id, intendedVersion: '1.0.0', sourceLanguage: 'en',
    title: 'Frozen integration foundations', description: 'Captured for Review.', objectives: ['Explain ownership'], changelog: 'Frozen submission.',
    license: { Kind: 'STANDARD', Identifier: 'CC-BY-4.0', DisplayName: 'Creative Commons Attribution 4.0', URL: '', CustomText: '' },
  },
  modules: [{
    id: structure.modules[0].id, stableKey: 'foundations', title: 'Frozen foundations', description: 'Historical module.', position: 0,
    lessons: [{
      id: lesson.id, stableKey: lesson.stable_key, title: 'Frozen lesson', description: 'Historical lesson.', objectives: ['Explain EAI'], estimatedDurationMinutes: 15, position: 0,
      prerequisiteStableKeys: [], content: { schemaVersion: 1, blocks: [{ key: 'snapshot-text', type: 'TEXT', payload: { content: { nodes: [{ type: 'paragraph', content: [{ type: 'text', text: 'Frozen semantic content.', marks: [] }] }] } } }] },
    }],
  }],
}

async function serveDraft(page: Page) {
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route(`**/api/authoring/drafts/${draftID}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(draft) }))
  await page.route(`**/api/authoring/drafts/${draftID}/structure`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ modules: [] }) }))
}

async function serveReviewOverview(page: Page) {
  await page.route(`**/api/authoring/drafts/${draftID}/reviews`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ reviews: [reviewCycle] }) }))
  await page.route(`**/api/authoring/drafts/${draftID}/reviews/active`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(reviewCycle) }))
  await page.route(`**/api/authoring/drafts/${draftID}/reviews/latest`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(reviewCycle) }))
}

async function serveApprovedReviewSnapshot(page: Page) {
  await serveDraft(page)
  await page.route(`**/api/authoring/drafts/${draftID}/members`, (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ members: [{ userId: session.user_id, role: 'MAINTAINER' }] }),
  }))
  await page.route(`**/api/authoring/drafts/${draftID}/reviews/${reviewCycle.id}`, (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ review: approvedReviewCycle, snapshot: reviewSnapshot }),
  }))
}

test('Authoring discovery is reachable from main navigation and opens an accessible Draft workspace', async ({ page }) => {
  await serveDraft(page)
  await page.route('**/api/authoring/drafts', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ drafts: [{ id: draftID, title: draft.title, intendedVersion: draft.intended_version, status: draft.status, updatedAt: draft.updated_at }] }),
  }))
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')
  await page.getByRole('link', { name: 'Authoring' }).focus()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL('/authoring')
  await expect(page.getByRole('link', { name: 'Authoring' })).toHaveAttribute('aria-current', 'page')
  await expect(page.getByRole('heading', { level: 1, name: 'Authoring' })).toBeVisible()
  await expect(page.getByRole('link', { name: `Open Draft: ${draft.title}` })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.setViewportSize({ width: 320, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await page.setViewportSize({ width: 390, height: 844 })
  await page.evaluate(() => { document.documentElement.style.fontSize = '200%' })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await page.evaluate(() => { document.documentElement.style.fontSize = '' })
  await page.getByRole('link', { name: `Open Draft: ${draft.title}` }).click()
  await expect(page).toHaveURL(`/authoring/drafts/${draftID}/overview`)
  await expect(page.getByRole('heading', { level: 1, name: draft.title })).toBeVisible()
  await page.goBack()
  await expect(page).toHaveURL('/authoring')
  await expect(page.getByRole('link', { name: `Open Draft: ${draft.title}` })).toBeVisible()
})

test('Authoring creates a Draft from the accessible home flow and opens its workspace', async ({ page }) => {
  const createdID = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'
  const created = { ...draft, id: createdID, course_id: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb', title: 'First Draft', revision: 1 }
  let createdVisible = false
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route('**/api/authoring/drafts', (route) => {
    if (route.request().method() === 'POST') {
      expect(route.request().headers()['x-csrf-token']).toBe('test-csrf-token')
      expect(route.request().postDataJSON()).toEqual({
        title: 'First Draft', intendedVersion: '0.1.0', sourceLanguage: 'en', description: 'A complete initial description.', objectives: ['Explain the first topic'], changelog: 'Initial Draft.',
      })
      createdVisible = true
      return route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(created) })
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ drafts: createdVisible ? [{ id: createdID, title: created.title, intendedVersion: created.intended_version, status: created.status, updatedAt: created.updated_at }] : [] }) })
  })
  await page.route(`**/api/authoring/drafts/${createdID}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(created) }))
  await page.route(`**/api/authoring/drafts/${createdID}/structure`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ modules: [] }) }))
  await page.goto('/authoring')
  await expect(page.getByText('You do not currently have any course Drafts.')).toBeVisible()
  await page.getByRole('button', { name: 'Create Draft' }).focus()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL('/authoring/new')
  await expect(page.getByRole('heading', { level: 1, name: 'Create Draft' })).toBeVisible()
  await page.getByRole('textbox', { name: /^Title required$/ }).fill('First Draft')
  await page.getByRole('textbox', { name: /^Description required$/ }).fill('A complete initial description.')
  await page.getByRole('textbox', { name: /^Learning objectives required$/ }).fill('Explain the first topic')
  await page.getByRole('button', { name: 'Create Draft' }).focus()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(`/authoring/drafts/${createdID}/overview`)
  await expect(page.getByRole('heading', { level: 1, name: created.title })).toBeVisible()
  await page.goBack()
  await expect(page).toHaveURL('/authoring/new')
  await page.goBack()
  await expect(page).toHaveURL('/authoring')
  await expect(page.getByRole('link', { name: `Open Draft: ${created.title}` })).toBeVisible()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('Authoring content edits canonical blocks with keyboard controls and preserves deferred payloads', async ({ page }) => {
  let contentBody: { expectedLessonRevision: number; content: { schemaVersion: number; blocks: { key: string; type: string; payload: unknown }[] } } | undefined
  const deferred = { key: 'architecture-diagram', type: 'IMAGE', payload: { asset: { assetKey: 'diagram' }, decorative: false, altText: 'Architecture diagram', caption: 'Reference' } }
  await serveDraft(page)
  await page.route(`**/api/authoring/drafts/${draftID}/lessons/${lesson.id}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ...lesson, content: { schemaVersion: 1, blocks: [deferred] } }) }))
  await page.route(`**/api/authoring/drafts/${draftID}/lessons/${lesson.id}/content`, (route) => {
    contentBody = route.request().postDataJSON()
    expect(route.request().method()).toBe('PUT')
    expect(route.request().headers()['x-csrf-token']).toBe('test-csrf-token')
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ lesson: { ...lesson, revision: 3, draftRevision: 4 }, content: contentBody?.content }) })
  })
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto(`/authoring/drafts/${draftID}/lessons/${lesson.id}`)
  await expect(page.getByRole('heading', { name: 'Lesson content' })).toBeVisible()
  await page.getByRole('combobox', { name: 'Block type' }).selectOption('TEXT')
  await page.getByRole('button', { name: 'Add block' }).focus()
  await page.keyboard.press('Enter')
  await page.getByRole('textbox', { name: 'Block 2 text · Paragraph 1 · Text 1' }).fill('A semantic lesson paragraph.')
  await page.getByRole('combobox', { name: 'Block type' }).selectOption('CODE')
  await page.getByRole('button', { name: 'Add block' }).focus()
  await page.keyboard.press('Enter')
  await page.getByRole('textbox', { name: 'Code', exact: true }).fill('  const message = "hello";\n')
  await page.getByRole('button', { name: 'Move block 3 Code up' }).focus()
  await page.keyboard.press('Enter')
  await page.getByRole('button', { name: 'Save Lesson content' }).focus()
  await page.keyboard.press('Enter')
  await expect(page.getByText('Lesson content saved.')).toBeVisible()
  expect(contentBody?.expectedLessonRevision).toBe(2)
  expect(contentBody?.content.blocks.map((block) => block.type)).toEqual(['IMAGE', 'CODE', 'TEXT'])
  expect(contentBody?.content.blocks[0]).toEqual(deferred)
  await expect(page.locator('img[src="diagram"]')).toHaveCount(0)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.screenshot({ path: '/tmp/btg-content-editor-desktop.png', fullPage: true })
  for (const width of [768, 390, 320]) {
    await page.setViewportSize({ width, height: 844 })
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  }
  await page.screenshot({ path: '/tmp/btg-content-editor-mobile.png', fullPage: true })
  await page.setViewportSize({ width: 768, height: 844 })
  await page.evaluate(() => { document.documentElement.style.fontSize = '200%' })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

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

test('Authoring Review overview is read-only, keyboard reachable, and responsive', async ({ page }) => {
  await serveDraft(page)
  await serveReviewOverview(page)
  let snapshotRequests = 0
  page.on('request', (request) => {
    if (request.url().includes(`/reviews/${reviewCycle.id}`)) snapshotRequests += 1
  })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/authoring/drafts/${draftID}/review`)
  await expect(page.getByRole('heading', { level: 2, name: 'Review' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Review', exact: true })).toHaveAttribute('aria-current', 'page')
  await expect(page.getByText('In review').first()).toBeVisible()
  await expect(page.getByRole('list', { name: 'Review history' })).toBeVisible()
  await expect(page.getByRole('button', { name: /submit|approve|request changes/i })).toHaveCount(0)
  expect(snapshotRequests).toBe(0)
  await page.getByRole('link', { name: 'Review', exact: true }).focus()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('Authoring Review history opens an exact frozen snapshot with read-only content', async ({ page }) => {
  let snapshotRequests = 0
  let structureRequests = 0
  await serveDraft(page)
  await serveReviewOverview(page)
  await page.route(`**/api/authoring/drafts/${draftID}/structure`, (route) => {
    structureRequests += 1
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(structure) })
  })
  await page.route(`**/api/authoring/drafts/${draftID}/reviews/${reviewCycle.id}`, (route) => {
    snapshotRequests += 1
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ review: reviewCycle, snapshot: reviewSnapshot }) })
  })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/authoring/drafts/${draftID}/review`)
  await page.getByRole('link', { name: 'View review' }).focus()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(`/authoring/drafts/${draftID}/reviews/${reviewCycle.id}`)
  await expect(page.getByRole('heading', { level: 2, name: 'Reviewing Draft revision 3' })).toBeVisible()
  await expect(page.getByText('Frozen integration foundations')).toBeVisible()
  await expect(page.getByText('Frozen semantic content.')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Approve' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Request changes' })).toBeVisible()
  await expect(page.getByRole('button', { name: /save|submit/i })).toHaveCount(0)
  expect(snapshotRequests).toBe(1)
  expect(structureRequests).toBe(0)
  for (const width of [320, 390]) {
    await page.setViewportSize({ width, height: 844 })
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  }
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('Authoring decides an in-review snapshot with the authoritative Review revision', async ({ page }) => {
  let review = { ...reviewCycle }
  let decisionBody: unknown
  await serveDraft(page)
  await page.route(`**/api/authoring/drafts/${draftID}/members`, (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ members: [{ userId: session.user_id, role: 'MAINTAINER' }] }),
  }))
  await page.route(`**/api/authoring/drafts/${draftID}/reviews/${reviewCycle.id}/approve`, (route) => {
    decisionBody = route.request().postDataJSON()
    expect(route.request().headers()['x-csrf-token']).toBe('test-csrf-token')
    review = {
      ...review,
      status: 'APPROVED',
      reviewRevision: 2,
      decidedBy: session.user_id,
      decidedAt: '2026-09-15T13:00:00Z',
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(review) })
  })
  await page.route(`**/api/authoring/drafts/${draftID}/reviews/${reviewCycle.id}`, (route) => {
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ review, snapshot: reviewSnapshot }) })
  })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/authoring/drafts/${draftID}/reviews/${reviewCycle.id}`)
  const approve = page.getByRole('button', { name: 'Approve' })
  await approve.focus()
  await page.keyboard.press('Enter')
  await expect(page.getByText('Approved Review is not a published Course.')).toBeVisible()
  expect(decisionBody).toEqual({ expectedReviewRevision: 1 })
  await expect(page.getByText('Frozen semantic content.')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Approve' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Request changes' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Publish course version' })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('Authoring publishes an approved Review with keyboard and focuses authoritative success', async ({ page }) => {
  let publicationBody: unknown
  let releasePublication: () => void = () => undefined
  const publicationGate = new Promise<void>((resolve) => { releasePublication = resolve })
  await serveApprovedReviewSnapshot(page)
  await page.route(`**/api/authoring/drafts/${draftID}/reviews/${reviewCycle.id}/publish`, async (route) => {
    publicationBody = route.request().postDataJSON()
    await publicationGate
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        reviewId: reviewCycle.id,
        reviewRevision: 2,
        courseId: draft.course_id,
        courseVersion: '1.0.0',
        courseVersionId: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
        publishedAt: '2026-09-16T14:30:00Z',
      }),
    })
  })

  await page.goto(`/authoring/drafts/${draftID}/reviews/${reviewCycle.id}`)
  const publish = page.getByRole('button', { name: 'Publish course version' })
  await publish.focus()
  await page.keyboard.press('Enter')
  await expect(page.getByText('Publishing course version…')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Publishing…' })).toBeDisabled()
  releasePublication()
  const success = page.getByRole('heading', { level: 4, name: 'Course version published' })
  await expect(success).toBeVisible()
  await expect(page.getByText('Course version 1.0.0 was published successfully.')).toBeVisible()
  await expect(success).toBeFocused()
  expect(publicationBody).toEqual({ expectedReviewRevision: 2 })
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('Authoring exposes and focuses blocking publication validation issues', async ({ page }) => {
  await serveApprovedReviewSnapshot(page)
  await page.route(`**/api/authoring/drafts/${draftID}/reviews/${reviewCycle.id}/publish`, (route) => route.fulfill({
    status: 422,
    contentType: 'application/problem+json',
    body: JSON.stringify({
      type: 'https://academy.example/problems/publication-validation-failed',
      title: 'Publication validation failed',
      status: 422,
      instance: route.request().url(),
      request_id: 'publication-validation-request',
      code: 'publication_validation_failed',
      issues: [
        { code: 'unresolved_asset_reference', path: 'modules[0].lessons[0].content.blocks[1]', message: 'The image asset is not available.' },
        { code: 'unresolved_assessment_reference', path: 'modules[0].lessons[0].content.blocks[2]', message: 'The assessment is not available.' },
      ],
    }),
  }))

  await page.goto(`/authoring/drafts/${draftID}/reviews/${reviewCycle.id}`)
  await page.getByRole('button', { name: 'Publish course version' }).focus()
  await page.keyboard.press('Enter')
  const summary = page.getByRole('heading', { level: 4, name: 'Publication validation issues' })
  await expect(summary).toBeVisible()
  await expect(page.getByText('The image asset is not available.')).toBeVisible()
  await expect(page.getByText('The assessment is not available.')).toBeVisible()
  await expect(summary).toBeFocused()
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('Authoring presents an independent-review policy conflict accessibly', async ({ page }) => {
  await serveDraft(page)
  await page.route(`**/api/authoring/drafts/${draftID}/reviews/${reviewCycle.id}/approve`, (route) => route.fulfill({
    status: 409,
    contentType: 'application/problem+json',
    body: JSON.stringify({
      type: 'https://academy.example/problems/independent-reviewer-required',
      title: 'Independent reviewer required',
      status: 409,
      instance: route.request().url(),
      request_id: 'policy-conflict-request',
      code: 'independent_reviewer_required',
    }),
  }))
  await page.route(`**/api/authoring/drafts/${draftID}/reviews/${reviewCycle.id}`, (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ review: reviewCycle, snapshot: reviewSnapshot }),
  }))

  await page.setViewportSize({ width: 320, height: 844 })
  await page.goto(`/authoring/drafts/${draftID}/reviews/${reviewCycle.id}`)
  await page.getByRole('button', { name: 'Approve' }).click()
  await expect(page.getByRole('alert')).toContainText('someone other than the person who submitted it')
  await expect(page.getByText('Frozen semantic content.')).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('Authoring submits the current Draft revision for Review and reloads authoritative state', async ({ page }) => {
  let reviews: typeof reviewCycle[] = []
  let submissionBody: unknown
  let snapshotReads = 0
  const submitted = { ...reviewCycle, draftRevision: 3 }
  await serveDraft(page)
  page.on('request', (request) => {
    if (request.url().includes(`/reviews/${reviewCycle.id}`)) snapshotReads += 1
  })
  await page.route(`**/api/authoring/drafts/${draftID}/reviews`, (route) => {
    if (route.request().method() === 'GET') return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ reviews }) })
    submissionBody = route.request().postDataJSON()
    expect(route.request().headers()['x-csrf-token']).toBe('test-csrf-token')
    reviews = [submitted]
    return route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({ review: submitted, snapshot: { schemaVersion: 1, draft: {}, modules: [] } }) })
  })
  await page.route(`**/api/authoring/drafts/${draftID}/reviews/active`, (route) => {
    if (!reviews.length) return route.fulfill({ status: 404, contentType: 'application/problem+json', body: JSON.stringify({}) })
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(reviews[0]) })
  })
  await page.route(`**/api/authoring/drafts/${draftID}/reviews/latest`, (route) => {
    if (!reviews.length) return route.fulfill({ status: 404, contentType: 'application/problem+json', body: JSON.stringify({}) })
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(reviews[0]) })
  })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/authoring/drafts/${draftID}/review`)
  const submit = page.getByRole('button', { name: 'Submit for review' })
  await submit.focus()
  await page.keyboard.press('Enter')
  await expect(page.getByText('Draft submitted for review.')).toBeVisible()
  expect(submissionBody).toEqual({ expectedDraftRevision: 3 })
  await expect(page.getByText('In review').first()).toBeVisible()
  await expect(page.getByRole('button', { name: 'Submit for review' })).toHaveCount(0)
  expect(snapshotReads).toBe(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
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

test('Authoring member management uses opaque IDs, authoritative reloads, and keyboard-accessible actions', async ({ page }) => {
  let revision = 3
  let activeMembers = [
    { userId: '66666666-6666-4666-8666-666666666666', role: 'AUTHOR' },
    { userId: '77777777-7777-4777-8777-777777777777', role: 'MAINTAINER' },
  ]
  const addedMember = '88888888-8888-4888-8888-888888888888'
  await page.route('**/api/auth/session', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(session) }))
  await page.route(`**/api/authoring/drafts/${draftID}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ...draft, revision }) }))
  await page.route(`**/api/authoring/drafts/${draftID}/members`, (route) => {
    if (route.request().method() === 'GET') return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ members: activeMembers }) })
    const body = route.request().postDataJSON() as { userId: string; role: string; expectedDraftRevision: number }
    expect(body).toEqual({ userId: addedMember, role: 'MAINTAINER', expectedDraftRevision: revision })
    expect(route.request().headers()['x-csrf-token']).toBe('test-csrf-token')
    activeMembers = [...activeMembers, { userId: body.userId, role: body.role }]
    revision += 1
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ draftRevision: revision }) })
  })
  await page.route(`**/api/authoring/drafts/${draftID}/members/*`, (route) => {
    const userID = route.request().url().split('/').pop()
    const body = route.request().postDataJSON() as { role?: string; expectedDraftRevision: number }
    expect(body.expectedDraftRevision).toBe(revision)
    if (route.request().method() === 'PATCH') activeMembers = activeMembers.map((member) => member.userId === userID ? { ...member, role: body.role! } : member)
    else activeMembers = activeMembers.filter((member) => member.userId !== userID)
    revision += 1
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ draftRevision: revision }) })
  })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(`/authoring/drafts/${draftID}/members`)
  await expect(page.getByRole('heading', { level: 2, name: 'Members' })).toBeVisible()
  await page.getByRole('textbox', { name: 'User ID' }).fill(addedMember)
  await page.getByRole('combobox', { name: /Role required/ }).selectOption('MAINTAINER')
  await page.getByRole('button', { name: 'Add member' }).focus()
  await page.keyboard.press('Enter')
  await expect(page.getByText('Member added.')).toBeVisible()
  await expect(page.getByText(addedMember, { exact: true })).toBeVisible()

  const authorRow = page.getByText('66666666-6666-4666-8666-666666666666', { exact: true }).locator('xpath=ancestor::li')
  await authorRow.getByRole('combobox').selectOption('MAINTAINER')
  await authorRow.getByRole('button', { name: 'Save role' }).focus()
  await page.keyboard.press('Enter')
  await expect(page.getByText('Member role updated.')).toBeVisible()

  const addedRow = page.getByText(addedMember, { exact: true }).locator('xpath=ancestor::li')
  await addedRow.getByRole('button', { name: `Revoke access for ${addedMember}` }).focus()
  await page.keyboard.press('Enter')
  await addedRow.getByRole('button', { name: `Confirm revoke access for ${addedMember}` }).focus()
  await page.keyboard.press('Enter')
  await expect(page.getByText('Member access revoked.')).toBeVisible()
  await expect(page.getByText(addedMember, { exact: true })).toHaveCount(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})

test('Authoring routes reflow at 320px and 390px with enlarged text', async ({ page }) => {
  await serveDraft(page)
  await serveReviewOverview(page)
  await page.route('**/api/authoring/drafts', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ drafts: [{ id: draftID, title: draft.title, intendedVersion: draft.intended_version, status: draft.status, updatedAt: draft.updated_at }] }) }))
  await page.route(`**/api/authoring/drafts/${draftID}/structure`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(structure) }))
  await page.route(`**/api/authoring/drafts/${draftID}/members`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ members: [{ userId: '77777777-7777-4777-8777-777777777777', role: 'MAINTAINER' }] }) }))
  await page.route(`**/api/authoring/drafts/${draftID}/lessons/${lesson.id}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ...lesson, content: { schemaVersion: 1, blocks: [{ key: 'long-stable-block-key-for-reflow', type: 'CODE', payload: { code: 'const example = "unbroken-content-that-must-not-widen-the-page";' } }] } }) }))
  await page.route(`**/api/authoring/drafts/${draftID}/reviews/${reviewCycle.id}`, (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ review: reviewCycle, snapshot: reviewSnapshot }) }))
  const routes = ['/authoring', `/authoring/drafts/${draftID}/overview`, `/authoring/drafts/${draftID}/structure`, `/authoring/drafts/${draftID}/lessons/${lesson.id}`, `/authoring/drafts/${draftID}/members`, `/authoring/drafts/${draftID}/review`, `/authoring/drafts/${draftID}/reviews/${reviewCycle.id}`]
  for (const path of routes) {
    await page.goto(path)
    await expect(page.getByRole('heading', { level: path === '/authoring' ? 1 : 2 }).first()).toBeVisible()
    if (path.includes('/lessons/')) await expect(page.getByRole('heading', { name: 'Lesson content' })).toBeVisible()
    for (const width of [320, 390]) {
      await page.setViewportSize({ width, height: 844 })
      for (const fontSize of ['100%', '200%']) {
        await page.evaluate((size) => { document.documentElement.style.fontSize = size }, fontSize)
        const overflow = await page.evaluate(() => ({
          width: document.documentElement.scrollWidth,
          offenders: [...document.querySelectorAll('*')].filter((el) => el.getBoundingClientRect().right > window.innerWidth + 1).map((el) => `${el.tagName}.${el.className}`).slice(0, 15),
        }))
        expect(overflow.width, `${path} at ${width}px / ${fontSize}: ${overflow.offenders.join(', ')}`).toBeLessThanOrEqual(width)
      }
    }
    expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  }
  await page.screenshot({ path: '/tmp/btg-authoring-hardening-members-reflow.png', fullPage: true })
})
