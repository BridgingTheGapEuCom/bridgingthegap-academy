import { describe, expect, it, vi } from 'vitest'
import { installPlugin, listPlugins, setPluginEnabled } from './management'

const release = { pluginId: 'com.example.widget', version: '1.0.0', artifactDigest: 'a'.repeat(64), name: 'Example', description: 'Safe', publisherName: 'Example', entrypoints: [], currentTrust: 'UNKNOWN', validationStatus: 'VALIDATED_AT_REGISTRATION', signatureStatus: 'UNSIGNED', approvalState: 'NONE', enabled: false, executionPermitted: false, installedAt: '2026-09-26T12:00:00Z' }

describe('plugin management API helpers', () => {
  it('uses authoritative management projections and a raw ZIP package body', async () => {
    const request = vi.fn().mockResolvedValue({ plugins: [release] })
    await expect(listPlugins({ request } as never)).resolves.toEqual({ plugins: [release] })

    const requestWithStatus = vi.fn().mockResolvedValue({ data: release, status: 201 })
    const file = new File(['zip'], 'example.zip', { type: 'application/zip' })
    await expect(installPlugin(file, { requestWithStatus })).resolves.toEqual({ release, replayed: false })
    expect(requestWithStatus).toHaveBeenCalledWith('/api/plugins', expect.objectContaining({ method: 'POST', body: file, headers: { 'Content-Type': 'application/zip' } }))
  })

  it('distinguishes the idempotent 200 replay and derives lifecycle paths from immutable coordinates', async () => {
    const requestWithStatus = vi.fn().mockResolvedValue({ data: release, status: 200 })
    await expect(installPlugin(new File(['zip'], 'example.zip'), { requestWithStatus })).resolves.toMatchObject({ replayed: true })
    const request = vi.fn().mockResolvedValue({ ...release, enabled: true })
    await setPluginEnabled(release, true, { request } as never)
    expect(request).toHaveBeenCalledWith('/api/plugins/com.example.widget/1.0.0/enable', { method: 'POST', cache: 'no-store' })
  })
})
