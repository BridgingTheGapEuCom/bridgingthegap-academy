import { describe, expect, it } from 'vitest'
import { APIProblemError } from '../api/client'
import { canDisplayAuthoringPublication, classifyAuthoringPublicationFailure } from './publication'

describe('publication mutation error classification', () => {
  it('offers publishing only to the current MAINTAINER in the visible membership projection', () => {
    const members = { members: [
      { userId: 'author', role: 'AUTHOR' as const },
      { userId: 'maintainer', role: 'MAINTAINER' as const },
    ] }
    expect(canDisplayAuthoringPublication(members, 'maintainer')).toBe(true)
    expect(canDisplayAuthoringPublication(members, 'author')).toBe(false)
    expect(canDisplayAuthoringPublication(members, 'someone-else')).toBe(false)
  })

  it('preserves structured validation issues without parsing safe messages', () => {
    const issues = [{ code: 'unresolved_asset_reference', path: 'modules[0].lessons[0]', message: 'Asset unavailable.' }]
    const failure = classifyAuthoringPublicationFailure(new APIProblemError(422, {
      type: 'https://academy.example/problems/publication-validation-failed', title: 'Publication validation failed', status: 422,
      instance: '/publish', request_id: 'request-id', code: 'publication_validation_failed', issues,
    }, undefined))
    expect(failure).toEqual({ kind: 'validation', issues })
  })

  it('retains distinct machine-readable publication conflicts', () => {
    const failure = classifyAuthoringPublicationFailure(new APIProblemError(409, {
      type: 'https://academy.example/problems/course-version-already-exists', title: 'Version conflict', status: 409,
      instance: '/publish', request_id: 'request-id', code: 'course_version_already_exists',
    }, undefined))
    expect(failure).toEqual({ kind: 'conflict', code: 'course_version_already_exists' })
  })
})
