import { describe, expect, it, vi } from 'vitest'
import { getLearnerCertificate, getPublicCertificate, issueCertificate } from './certificates'

const course = '11111111-1111-4111-8111-111111111111'
const id = '22222222-2222-4222-8222-222222222222'
const achievement = { courseId: course, title: 'Course', version: '1.0.0', language: 'en', criteria: 'Finished' }
const issuer = { id: 'https://academy.example', name: 'Academy' }
const privateCertificate = { certificateId: id, status: 'ACTIVE', issuedAt: '2026-01-01T00:00:00Z', revokedAt: null, achievement, issuer, verificationPath: `/verify/certificates/${id}` }
const publicCertificate = (({ verificationPath: _verificationPath, ...certificate }) => certificate)(privateCertificate)

describe('certificate API helpers', () => {
  it('uses the server-owned issue route and validates the private certificate', async () => {
    const request = vi.fn().mockResolvedValue(privateCertificate)
    await expect(issueCertificate(course, '1.0.0', { request } as never)).resolves.toEqual(privateCertificate)
    expect(request).toHaveBeenCalledWith(`/api/courses/by-id/${course}/versions/1.0.0/certificate`, { method: 'POST', cache: 'no-store' })
  })

  it('keeps the public recipient-free contract strict', async () => {
    await expect(getPublicCertificate(id, { request: vi.fn().mockResolvedValue(publicCertificate) } as never)).resolves.toEqual(publicCertificate)
    await expect(getPublicCertificate(id, { request: vi.fn().mockResolvedValue(privateCertificate) } as never)).rejects.toThrow()
    await expect(getPublicCertificate(id, { request: vi.fn().mockResolvedValue({ ...publicCertificate, email: 'private@example.com' }) } as never)).rejects.toThrow()
  })

  it('validates lifecycle state and learner reads', async () => {
    const revoked = { ...privateCertificate, status: 'REVOKED', revokedAt: '2026-01-02T00:00:00Z' }
    await expect(getLearnerCertificate(id, { request: vi.fn().mockResolvedValue(revoked) } as never)).resolves.toMatchObject({ status: 'REVOKED' })
    await expect(getLearnerCertificate(id, { request: vi.fn().mockResolvedValue({ ...revoked, revokedAt: null }) } as never)).rejects.toThrow()
  })

  it('accepts an optional server-authoritative signed badge path only for learner reads', async () => {
    const openBadgePath = `/api/public/open-badges/${id}`
    await expect(getLearnerCertificate(id, { request: vi.fn().mockResolvedValue({ ...privateCertificate, openBadgePath }) } as never)).resolves.toMatchObject({ openBadgePath })
    await expect(getLearnerCertificate(id, { request: vi.fn().mockResolvedValue({ ...privateCertificate, openBadgePath: 42 }) } as never)).rejects.toThrow()
  })
})
