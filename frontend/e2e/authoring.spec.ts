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
  await page.getByRole('link', { name: `Open Draft: ${draft.title}` }).click()
  await expect(page).toHaveURL(`/authoring/drafts/${draftID}/overview`)
  await expect(page.getByRole('heading', { level: 1, name: draft.title })).toBeVisible()
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
  await expect(page.getByRole('link', { name: 'Review' })).toHaveAttribute('aria-current', 'page')
  await expect(page.getByText('In review').first()).toBeVisible()
  await expect(page.getByRole('list', { name: 'Review history' })).toBeVisible()
  await expect(page.getByRole('button', { name: /submit|approve|request changes/i })).toHaveCount(0)
  expect(snapshotRequests).toBe(0)
  await page.getByRole('link', { name: 'Review' }).focus()
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
  const routes = ['/authoring', `/authoring/drafts/${draftID}/overview`, `/authoring/drafts/${draftID}/structure`, `/authoring/drafts/${draftID}/lessons/${lesson.id}`, `/authoring/drafts/${draftID}/members`, `/authoring/drafts/${draftID}/review`]
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
