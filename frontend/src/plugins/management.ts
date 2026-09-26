import { APIProblemError, type APIClient } from '../api/client'
import type { components } from '../api/generated'
import { useAuth, type AuthService } from '../auth/auth'

export type PluginRelease = components['schemas']['PluginManagementRelease']
export type PluginReleaseList = components['schemas']['PluginManagementReleaseList']
export type PluginVerificationKey = components['schemas']['PluginVerificationKey']
export type PluginVerificationKeyList = components['schemas']['PluginVerificationKeyList']
export type PluginVerificationKeyCreate = components['schemas']['PluginVerificationKeyCreateRequest']

export class InvalidPluginManagementResponseError extends Error {
  constructor() { super('Invalid plugin management response') }
}

function object(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
function string(value: unknown): value is string { return typeof value === 'string' }
function bool(value: unknown): value is boolean { return typeof value === 'boolean' }
function array(value: unknown): value is unknown[] { return Array.isArray(value) }
function release(value: unknown): value is PluginRelease {
  if (!object(value)) return false
  return string(value.pluginId) && string(value.version) && string(value.artifactDigest) && string(value.name)
    && string(value.description) && string(value.publisherName) && array(value.entrypoints)
    && string(value.validationStatus) && string(value.signatureStatus) && string(value.approvalState)
    && bool(value.enabled) && bool(value.executionPermitted) && string(value.installedAt)
}
function releases(value: unknown): value is PluginReleaseList { return object(value) && array(value.plugins) && value.plugins.every(release) }
function key(value: unknown): value is PluginVerificationKey {
  return object(value) && string(value.keyId) && string(value.purpose) && array(value.allowedPluginIds)
    && bool(value.enabled) && string(value.fingerprint)
}
function keys(value: unknown): value is PluginVerificationKeyList { return object(value) && array(value.keys) && value.keys.every(key) }
function encoded(value: string): string { return encodeURIComponent(value) }
function releasePath(release: Pick<PluginRelease, 'pluginId' | 'version'>): string { return `/api/plugins/${encoded(release.pluginId)}/${encoded(release.version)}` }
function emptyPost() { return { method: 'POST', cache: 'no-store' as RequestCache } }
function json(method: string, body: unknown) { return { method, cache: 'no-store' as RequestCache, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } }

export async function listPlugins(client: Pick<AuthService, 'request'> = useAuth()): Promise<PluginReleaseList> {
  const value = await client.request<unknown>('/api/plugins', { cache: 'no-store' })
  if (!releases(value)) throw new InvalidPluginManagementResponseError()
  return value
}
export async function getPlugin(releaseIdentity: Pick<PluginRelease, 'pluginId' | 'version'>, client: Pick<AuthService, 'request'> = useAuth()): Promise<PluginRelease> {
  const value = await client.request<unknown>(releasePath(releaseIdentity), { cache: 'no-store' })
  if (!release(value)) throw new InvalidPluginManagementResponseError()
  return value
}
export async function installPlugin(file: File, client: { requestWithStatus<T>(path: string, options?: Parameters<APIClient['request']>[1]): Promise<{ data: T; status: number }> } = useAuth()): Promise<{ release: PluginRelease; replayed: boolean }> {
  const response = await client.requestWithStatus<unknown>('/api/plugins', { method: 'POST', cache: 'no-store', headers: { 'Content-Type': 'application/zip' }, body: file })
  const value = response.data
  if (!release(value)) throw new InvalidPluginManagementResponseError()
  return { release: value, replayed: response.status === 200 }
}
export async function approvePlugin(releaseIdentity: Pick<PluginRelease, 'pluginId' | 'version'>, client: Pick<AuthService, 'request'> = useAuth()): Promise<PluginRelease> {
  const value = await client.request<unknown>(`${releasePath(releaseIdentity)}/approval`, emptyPost())
  if (!release(value)) throw new InvalidPluginManagementResponseError()
  return value
}
export async function revokePluginApproval(releaseIdentity: Pick<PluginRelease, 'pluginId' | 'version'>, client: Pick<AuthService, 'request'> = useAuth()): Promise<PluginRelease> {
  const value = await client.request<unknown>(`${releasePath(releaseIdentity)}/approval`, { method: 'DELETE', cache: 'no-store', expectJSON: true })
  if (!release(value)) throw new InvalidPluginManagementResponseError()
  return value
}
export async function setPluginEnabled(releaseIdentity: Pick<PluginRelease, 'pluginId' | 'version'>, enabled: boolean, client: Pick<AuthService, 'request'> = useAuth()): Promise<PluginRelease> {
  const value = await client.request<unknown>(`${releasePath(releaseIdentity)}/${enabled ? 'enable' : 'disable'}`, emptyPost())
  if (!release(value)) throw new InvalidPluginManagementResponseError()
  return value
}
export async function listPluginKeys(client: Pick<AuthService, 'request'> = useAuth()): Promise<PluginVerificationKeyList> {
  const value = await client.request<unknown>('/api/plugins/keys', { cache: 'no-store' })
  if (!keys(value)) throw new InvalidPluginManagementResponseError()
  return value
}
export async function addPluginKey(input: PluginVerificationKeyCreate, client: Pick<AuthService, 'request'> = useAuth()): Promise<PluginVerificationKey> {
  const value = await client.request<unknown>('/api/plugins/keys', json('POST', input))
  if (!key(value)) throw new InvalidPluginManagementResponseError()
  return value
}
export async function setPluginKeyEnabled(keyID: string, enabled: boolean, client: Pick<AuthService, 'request'> = useAuth()): Promise<PluginVerificationKey> {
  const value = await client.request<unknown>(`/api/plugins/keys/${encoded(keyID)}/${enabled ? 'enable' : 'disable'}`, emptyPost())
  if (!key(value)) throw new InvalidPluginManagementResponseError()
  return value
}
export function isPluginManagementForbidden(error: unknown): boolean { return error instanceof APIProblemError && (error.status === 401 || error.status === 403) }
export function pluginManagementMessage(error: unknown, fallback: string): string {
  return error instanceof APIProblemError && error.problem?.title ? error.problem.title : fallback
}
