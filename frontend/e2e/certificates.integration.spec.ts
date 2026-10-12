import { execFileSync } from 'node:child_process'
import { expect, test } from '@playwright/test'

const email = 'learner-e2e@example.com'
const password = 'learner integration password'
const courseTitle = 'Learner reader integration'

function seed() {
  const output = execFileSync('go', ['run', './tools/learner-e2e-seed'], {
    cwd: process.cwd(), encoding: 'utf8', env: { ...process.env, BTG_LEARNER_E2E_ALLOW_SEED: '1' }, stdio: ['ignore', 'pipe', 'inherit'],
  })
  return JSON.parse(output) as { courseId: string; version: string }
}

function certificateStatus() {
  const output = execFileSync('go', ['run', './tools/certificate-e2e-seed'], {
    cwd: process.cwd(), encoding: 'utf8', env: { ...process.env, BTG_CERTIFICATE_E2E_ALLOW_INSPECT: '1' }, stdio: ['ignore', 'pipe', 'inherit'],
  })
  return JSON.parse(output) as { exists: boolean; certificateId?: string; status?: string; courseId?: string; count: number }
}

test('real issuance persists and public verification remains anonymous', async ({ page, browser }) => {
  const fixture = seed()
  expect(certificateStatus()).toMatchObject({ exists: false, count: 0 })

  await page.goto(`/login?returnTo=/courses/by-id/${fixture.courseId}/versions/${fixture.version}`)
  await page.getByLabel(/^Email/).fill(email)
  await page.getByLabel(/^Password/).fill(password)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('heading', { level: 1, name: courseTitle })).toBeVisible()

  await page.getByRole('button', { name: 'Mark lesson complete' }).click()
  await expect(page.getByText('Lesson complete', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Next lesson' }).click()
  await expect(page.getByRole('heading', { level: 2, name: 'Next steps' })).toBeVisible()
  await page.getByRole('button', { name: 'Mark lesson complete' }).click()
  await expect(page.getByText('2 of 2 lessons complete')).toBeVisible()

  const issuance = page.waitForResponse((response) => response.request().method() === 'POST' && response.url().endsWith('/certificate'))
  await page.getByRole('button', { name: 'Certificate' }).click()
  expect((await issuance).status()).toBe(201)
  await expect(page).toHaveURL(/\/certificates\/[0-9a-f-]+$/)
  await expect(page.getByRole('heading', { level: 1, name: courseTitle })).toBeVisible()
  await expect(page.getByText('Valid', { exact: true })).toBeVisible()
  const certificateID = page.url().split('/').at(-1)!

  await page.reload()
  await expect(page.getByText(certificateID)).toBeVisible()
  expect(certificateStatus()).toMatchObject({ exists: true, certificateId: certificateID, status: 'ACTIVE', courseId: fixture.courseId, count: 1 })

  const anonymous = await browser.newContext()
  const publicPage = await anonymous.newPage()
  await publicPage.goto(`/verify/certificates/${certificateID}`)
  await expect(publicPage.getByText('Valid certificate')).toBeVisible()
  await expect(publicPage.getByRole('heading', { level: 2, name: courseTitle })).toBeVisible()
  await expect(publicPage.getByText(/email|user id|learner id/i)).toHaveCount(0)

  await publicPage.goto('/verify/certificates/99999999-9999-4999-8999-999999999999')
  await expect(publicPage.getByRole('heading', { level: 1, name: 'Certificate not found' })).toBeVisible()
  await anonymous.close()
})
