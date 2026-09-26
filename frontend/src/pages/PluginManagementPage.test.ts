import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const listPluginsMock = vi.hoisted(() => vi.fn()); const listKeysMock = vi.hoisted(() => vi.fn()); const installMock = vi.hoisted(() => vi.fn()); const approveMock = vi.hoisted(() => vi.fn()); const revokeMock = vi.hoisted(() => vi.fn()); const enableMock = vi.hoisted(() => vi.fn()); const addKeyMock = vi.hoisted(() => vi.fn()); const keyEnabledMock = vi.hoisted(() => vi.fn()); const detailMock = vi.hoisted(() => vi.fn())
vi.mock('../plugins/management', () => ({ listPlugins: listPluginsMock, listPluginKeys: listKeysMock, installPlugin: installMock, approvePlugin: approveMock, revokePluginApproval: revokeMock, setPluginEnabled: enableMock, addPluginKey: addKeyMock, setPluginKeyEnabled: keyEnabledMock, getPlugin: detailMock, isPluginManagementForbidden: () => false, pluginManagementMessage: (_error: unknown, fallback: string) => fallback }))
import PluginManagementPage from './PluginManagementPage.vue'

const valid = { pluginId: 'com.example.valid', version: '1.0.0', artifactDigest: 'a'.repeat(64), name: 'Valid widget', description: 'A valid widget', publisherName: 'Example', entrypoints: [{ id: 'dashboard', type: 'DASHBOARD_WIDGET', name: 'Dashboard widget' }], currentTrust: 'UNKNOWN', validationStatus: 'VALIDATED_AT_REGISTRATION', signatureStatus: 'UNSIGNED', approvalState: 'NONE', enabled: false, executionPermitted: false, installedAt: '2026-09-26T12:00:00Z' }
const invalid = { ...valid, pluginId: 'com.example.invalid', name: 'Invalid signature', currentTrust: undefined, validationStatus: 'SIGNATURE_INVALID', signatureStatus: 'INVALID' }
const key = { keyId: 'approval-1', purpose: 'BTG_APPROVAL_SIGNING', allowedPluginIds: [], enabled: true, fingerprint: 'sha256:abc' }

describe('PluginManagementPage', () => {
  beforeEach(() => { for (const mock of [listPluginsMock, listKeysMock, installMock, approveMock, revokeMock, enableMock, addKeyMock, keyEnabledMock, detailMock]) mock.mockReset(); listPluginsMock.mockResolvedValue({ plugins: [valid, invalid] }); listKeysMock.mockResolvedValue({ keys: [key] }); detailMock.mockResolvedValue(valid) })
  afterEach(cleanup)

  it('keeps validation, trust, approval, enablement, and execution eligibility distinct', async () => {
    render(PluginManagementPage)
    await screen.findByText('Installed releases')
    expect(screen.getByText('This is a valid installed release that is not currently BTG-owned or BTG-approved.')).toBeTruthy()
    expect(screen.getByText('The recognized signature is invalid. This is not an UNKNOWN trust state.')).toBeTruthy()
    expect(screen.getAllByText('Stored enablement')).toHaveLength(2)
    expect(screen.getAllByText('Execution eligible now')).toHaveLength(2)
  })

  it('uploads the selected ZIP without client-selected release coordinates and reloads inventory', async () => {
    installMock.mockResolvedValue({ release: valid, replayed: false })
    render(PluginManagementPage); await screen.findByText('Install plugin package')
    const input = screen.getByLabelText(/Plugin ZIP package/)
    await fireEvent.change(input, { target: { files: [new File(['zip'], 'widget.zip', { type: 'application/zip' })] } })
    await fireEvent.click(screen.getByRole('button', { name: 'Install plugin' }))
    await waitFor(() => expect(installMock).toHaveBeenCalledWith(expect.any(File)))
    expect(listPluginsMock.mock.calls.length).toBeGreaterThan(1)
  })

  it('uses lifecycle endpoints and refreshes authoritative inventory after key changes', async () => {
    approveMock.mockResolvedValue({ ...valid, approvalState: 'ACTIVE', currentTrust: 'BTG_APPROVED' }); addKeyMock.mockResolvedValue(key)
    render(PluginManagementPage); await screen.findByText('Installed releases')
    await fireEvent.click(screen.getAllByRole('button', { name: 'Approve' })[0]!)
    await waitFor(() => expect(approveMock).toHaveBeenCalledWith(expect.objectContaining({ pluginId: valid.pluginId, version: valid.version })))
    await fireEvent.update(screen.getByLabelText(/Key ID/), 'owned-1'); await fireEvent.update(screen.getByLabelText(/Public key \(unpadded base64url\)/), 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'); await fireEvent.update(screen.getByLabelText(/Allowed plugin IDs/), 'com.example.valid')
    await fireEvent.click(screen.getByRole('button', { name: 'Add verification key' }))
    await waitFor(() => expect(addKeyMock).toHaveBeenCalledWith(expect.objectContaining({ allowedPluginIds: ['com.example.valid'] })))
    expect(listPluginsMock.mock.calls.length).toBeGreaterThan(2)
  })

  it('reloads authoritative state when a lifecycle operation is rejected', async () => {
    approveMock.mockRejectedValue(new Error('stale state'))
    render(PluginManagementPage); await screen.findByText('Installed releases')
    const initialLoads = listPluginsMock.mock.calls.length
    await fireEvent.click(screen.getAllByRole('button', { name: 'Approve' })[0]!)
    await waitFor(() => expect(listPluginsMock.mock.calls.length).toBeGreaterThan(initialLoads))
    expect(screen.getByRole('alert').textContent).toContain('could not be completed')
  })

  it('reloads authoritative state when verification-key registration is rejected', async () => {
    addKeyMock.mockRejectedValue(new Error('duplicate key'))
    render(PluginManagementPage); await screen.findByText('Verification keys')
    const initialLoads = listPluginsMock.mock.calls.length
    await fireEvent.update(screen.getByLabelText(/Key ID/), 'owned-1'); await fireEvent.update(screen.getByLabelText(/Public key \(unpadded base64url\)/), 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'); await fireEvent.update(screen.getByLabelText(/Allowed plugin IDs/), 'com.example.valid')
    await fireEvent.click(screen.getByRole('button', { name: 'Add verification key' }))
    await waitFor(() => expect(listPluginsMock.mock.calls.length).toBeGreaterThan(initialLoads))
    expect(screen.getByRole('alert').textContent).toContain('could not be added')
    expect((screen.getByLabelText(/Public key \(unpadded base64url\)/) as HTMLTextAreaElement).value).toBe('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')
  })
})
