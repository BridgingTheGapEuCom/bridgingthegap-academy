import { describe, expect, it } from 'vitest'
import { APIProblemError } from '../api/client'
import { InvalidCoursePackageResponseError, classifyCoursePackageFailure, formatBytes, packageErrorMessage, previewCoursePackage } from './portability'

const preview = { previewToken: 'a'.repeat(64), preview: { format: 'bridging-the-gap-course', formatVersion: 1, title: 'Course', version: '1.2.3', language: 'en', license: { kind: 'STANDARD', displayName: 'CC BY' }, attribution: [{ displayName: 'Author', role: 'AUTHOR', order: 0 }], moduleCount: 1, lessonCount: 2, assessmentCount: 1, assetCount: 1, assetBytes: 1536, translationLanguages: ['es'] } }

describe('course portability client', () => {
  it('strictly validates safe previews and sends the package as application/zip', async () => {
    const request = async <T>(_path: string, options?: { method?: string; body?: BodyInit | null; headers?: HeadersInit }) => {
      expect(options?.method).toBe('POST'); expect(options?.headers).toEqual({ 'Content-Type': 'application/zip' }); return preview as T
    }
    await expect(previewCoursePackage(new File(['zip'], 'course.zip', { type: 'application/zip' }), { request })).resolves.toEqual(preview)
    await expect(previewCoursePackage(new File(['zip'], 'course.zip'), { request: async <T>() => ({ previewToken: 'bad' } as T) })).rejects.toBeInstanceOf(InvalidCoursePackageResponseError)
  })

  it('maps safe parser and lifecycle failures without exposing API text', () => {
    expect(formatBytes(1536)).toBe('1.5 KB')
    expect(packageErrorMessage(new APIProblemError(422, { type: 'x', title: 'raw', status: 422, instance: '/', request_id: 'id', code: 'unsupported_package_version' }, undefined))).toContain('format version')
    expect(classifyCoursePackageFailure(new APIProblemError(404, undefined, undefined))).toBe('expired')
    expect(classifyCoursePackageFailure(new APIProblemError(409, { type: 'x', title: 'raw', status: 409, instance: '/', request_id: 'id', code: 'package_source_version_conflict' }, undefined))).toBe('conflict')
  })
})
