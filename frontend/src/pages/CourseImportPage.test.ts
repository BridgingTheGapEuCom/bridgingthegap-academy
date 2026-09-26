import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

const previewCoursePackageMock = vi.hoisted(() => vi.fn())
const executeCoursePackageMock = vi.hoisted(() => vi.fn())
vi.mock('../portability/portability', async (original) => ({ ...(await original<typeof import('../portability/portability')>()), previewCoursePackage: previewCoursePackageMock, executeCoursePackage: executeCoursePackageMock }))
import CourseImportPage from './CourseImportPage.vue'

const preview = { previewToken: 'a'.repeat(64), preview: { format: 'bridging-the-gap-course', formatVersion: 1, title: 'Imported course', version: '1.2.3', language: 'en', license: { kind: 'STANDARD', displayName: 'CC BY' }, attribution: [{ displayName: 'Public author', role: 'AUTHOR', order: 0 }], moduleCount: 1, lessonCount: 2, assessmentCount: 1, assetCount: 1, assetBytes: 1536, translationLanguages: ['es'] } }
const imported = { importId: '11111111-1111-4111-8111-111111111111', courseId: '22222222-2222-4222-8222-222222222222', courseVersionId: '33333333-3333-4333-8333-333333333333', semVer: '1.2.3', status: 'IMPORTED', translationLanguages: ['es'] }

async function renderPage() { const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/admin/course-import', component: CourseImportPage }, { path: '/courses/by-id/:courseId/versions/:version', component: { template: '<p>Course</p>' } }] }); await router.push('/admin/course-import'); await router.isReady(); return render({ template: '<RouterView />' }, { global: { plugins: [router] } }) }
async function choose(file = new File(['zip'], 'course.zip', { type: 'application/zip' })) {
  const input = screen.getByLabelText('Course package') as HTMLInputElement
  Object.defineProperty(input, 'files', { configurable: true, value: [file] })
  await fireEvent.change(input)
}

describe('CourseImportPage', () => {
  beforeEach(() => { previewCoursePackageMock.mockReset(); executeCoursePackageMock.mockReset(); previewCoursePackageMock.mockResolvedValue(preview); executeCoursePackageMock.mockResolvedValue(imported) })
  afterEach(cleanup)
  it('uses an explicit native file selection, preview, and import confirmation', async () => {
    await renderPage(); expect((screen.getByRole('button', { name: 'Validate package' }) as HTMLButtonElement).disabled).toBe(true); await choose(); expect(screen.getAllByText(/course.zip/).length).toBeGreaterThan(0); await fireEvent.click(screen.getByRole('button', { name: 'Validate package' })); await screen.findByRole('heading', { name: 'Review course package' }); expect(screen.getByText('Public author (AUTHOR)')).toBeTruthy(); expect(screen.queryByText(/correct answer/i)).toBeNull(); await fireEvent.click(screen.getByRole('button', { name: 'Import course' })); await screen.findByRole('heading', { name: 'Course imported' }); expect(screen.getByRole('link', { name: 'View imported course' }).getAttribute('href')).toContain('/courses/by-id/22222222-2222-4222-8222-222222222222/versions/1.2.3')
  })
  it('keeps preview for an operational retry, clears it for expiry, and shows conflict without overwrite', async () => {
    executeCoursePackageMock.mockRejectedValueOnce(new Error('temporary')).mockRejectedValueOnce({ status: 404 }).mockRejectedValueOnce({ status: 409 })
    await renderPage(); await choose(); await fireEvent.click(screen.getByRole('button', { name: 'Validate package' })); await screen.findByRole('heading', { name: 'Review course package' }); await fireEvent.click(screen.getByRole('button', { name: 'Import course' })); await waitFor(() => expect(screen.getByText(/You can try again/)).toBeTruthy()); expect(screen.getByRole('button', { name: 'Import course' })).toBeTruthy()
  })
  it('does not allow a late preview response to replace a newer selected file', async () => {
    let resolveOld: (value: typeof preview) => void = () => undefined; previewCoursePackageMock.mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve })).mockResolvedValueOnce({ ...preview, preview: { ...preview.preview, title: 'New course' } }); await renderPage(); await choose(new File(['a'], 'old.zip')); await fireEvent.click(screen.getByRole('button', { name: 'Validate package' })); await choose(new File(['b'], 'new.zip')); await fireEvent.click(screen.getByRole('button', { name: 'Validate package' })); await screen.findByText('New course'); resolveOld(preview); await new Promise((resolve) => setTimeout(resolve, 0)); expect(screen.getByText('New course')).toBeTruthy()
  })
})
