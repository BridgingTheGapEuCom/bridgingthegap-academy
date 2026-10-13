import { execFileSync } from 'node:child_process'
import { expect, test } from '@playwright/test'

type Fixture = { email: string; password: string; pluginPath: string; coursePackagePath: string; pluginId: string; pluginVersion: string; courseTitle: string }
type Status = { pluginInstalled: boolean; pluginApproved: boolean; pluginEnabled: boolean; importExists: boolean; importedCourseId: string }

function fixture(command: 'seed' | 'status'): Fixture & Status {
  const output = execFileSync('go', ['run', './tools/admin-e2e-seed', command], {
    cwd: process.cwd(), encoding: 'utf8', env: { ...process.env, BTG_ADMIN_E2E_ALLOW_SEED: '1' }, stdio: ['ignore', 'pipe', 'inherit'],
  })
  return JSON.parse(output) as Fixture & Status
}

test('real admin installs and manages a plugin and imports a course package', async ({ page }) => {
  const seeded = fixture('seed')
  await page.goto('/login?returnTo=/admin/plugins')
  await page.getByLabel(/^Email/).fill(seeded.email)
  await page.getByLabel(/^Password/).fill(seeded.password)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'Plugin management' })).toBeVisible()

  await page.getByLabel(/Plugin ZIP package/).setInputFiles(seeded.pluginPath)
  await page.getByRole('button', { name: 'Install plugin' }).click()
  await expect(page.getByRole('heading', { level: 3, name: /Admin integration plugin/ })).toBeVisible()
  await expect(page.getByText('Unknown trust', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Approve' }).click()
  await expect(page.getByText('BTG approved', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Enable', exact: true }).click()
  await expect(page.getByText('Enabled', { exact: true }).first()).toBeVisible()
  await page.reload()
  await expect(page.getByRole('button', { name: 'Disable', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Disable', exact: true }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.getByRole('dialog').getByRole('button', { name: 'Disable release' }).click()
  await page.reload()
  await expect(page.getByRole('button', { name: 'Enable', exact: true })).toBeVisible()

  await page.goto('/admin/course-import')
  await page.getByLabel(/Course package/).setInputFiles(seeded.coursePackagePath)
  await page.getByRole('button', { name: 'Validate package' }).click()
  await expect(page.getByRole('heading', { name: 'Review course package' })).toBeVisible()
  await expect(page.getByText(seeded.courseTitle)).toBeVisible()
  await page.getByRole('button', { name: 'Import course' }).click()
  await expect(page.getByRole('heading', { name: /Course imported|Already imported/ })).toBeVisible()
  await page.getByRole('link', { name: 'View imported course' }).click()
  await expect(page.getByRole('heading', { level: 1, name: seeded.courseTitle })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('heading', { level: 1, name: seeded.courseTitle })).toBeVisible()

  expect(fixture('status')).toMatchObject({ pluginInstalled: true, pluginApproved: true, pluginEnabled: false, importExists: true })
})
