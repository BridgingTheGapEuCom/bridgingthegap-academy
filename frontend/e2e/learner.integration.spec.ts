import { expect, test } from '@playwright/test'

const enabled = process.env.BTG_LEARNER_E2E_REAL === '1'
const email = process.env.BTG_LEARNER_E2E_EMAIL ?? 'learner-e2e@example.com'
const password = process.env.BTG_LEARNER_E2E_PASSWORD ?? 'learner integration password'
const courseID = process.env.BTG_LEARNER_E2E_COURSE_ID
const courseTitle = process.env.BTG_LEARNER_E2E_COURSE_TITLE ?? 'Learner reader integration'
const version = process.env.BTG_LEARNER_E2E_VERSION ?? '1.0.0'

test('real learner progress and grading persist across reload', async ({ page }) => {
  test.skip(!enabled || !courseID, 'Seed a real test stack and set BTG_LEARNER_E2E_REAL=1 plus BTG_LEARNER_E2E_COURSE_ID')

  await page.goto('/login?returnTo=/courses')
  await page.getByLabel(/^Email/).fill(email)
  await page.getByLabel(/^Password/).fill(password)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page).toHaveURL('/courses')

  await expect(page.getByRole('link', { name: courseTitle })).toBeVisible()
  await page.getByRole('link', { name: courseTitle }).click()
  await expect(page).toHaveURL(new RegExp(`/courses/by-id/${courseID}`))
  await expect(page.getByRole('heading', { level: 2, name: 'Introduction' })).toBeVisible()
  await expect(page.getByText('Real-stack learner content.')).toBeVisible()

  const progressResponse = page.waitForResponse((response) => response.request().method() === 'PUT' && response.url().endsWith('/progress/lessons/introduction'))
  await page.getByRole('button', { name: 'Mark lesson complete' }).click()
  expect((await progressResponse).status()).toBe(200)
  await expect(page.getByText('Lesson complete', { exact: true })).toBeVisible()

  await page.getByRole('radio', { name: 'The Academy backend' }).check()
  const submitResponse = page.waitForResponse((response) => response.request().method() === 'POST' && response.url().includes('/api/learner/assessment-attempts/') && response.url().endsWith('/submit'))
  await page.getByRole('button', { name: 'Submit answers' }).click()
  const submitted = await submitResponse
  expect(submitted.status()).toBe(200)
  const attempt = await submitted.json() as { attemptId: string; assessmentKey: string; state: string; result: { correctCount: number; totalCount: number; percentage: number } }
  expect(attempt.state).toBe('SUBMITTED')
  expect(attempt.result).toEqual({ correctCount: 1, totalCount: 1, percentage: 100 })
  await expect(page.getByRole('heading', { name: 'Submitted result' })).toBeVisible()

  await page.reload()
  await expect(page.getByText('Lesson complete', { exact: true })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Submitted result' })).toBeVisible()
  await expect(page.getByText(/1 of 1 questions correct/)).toBeVisible()

  const progress = await page.request.get(`/api/courses/by-id/${courseID}/versions/${version}/progress`)
  expect(progress.status()).toBe(200)
  expect(await progress.json()).toMatchObject({ completedLessonKeys: ['introduction'] })
  const latestAttempt = await page.request.get(`/api/courses/by-id/${courseID}/versions/${version}/assessments/${attempt.assessmentKey}/attempts`)
  expect(latestAttempt.status()).toBe(200)
  expect(await latestAttempt.json()).toMatchObject({ attemptId: attempt.attemptId, state: 'SUBMITTED', result: { correctCount: 1, totalCount: 1 } })
})
