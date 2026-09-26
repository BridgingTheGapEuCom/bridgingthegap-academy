import { APIProblemError, type APIClient } from '../api/client'
import type { components } from '../api/generated'
import { useAuth, type AuthService } from '../auth/auth'
import type { WidgetRuntimeLaunch } from '../plugins/runtime'

export type DashboardWidgetPlacement = components['schemas']['DashboardWidgetPlacement']
export type DashboardWidgetPlacementList = components['schemas']['DashboardWidgetPlacementList']
export type AvailableDashboardWidget = components['schemas']['AvailableDashboardWidget']
export type AvailableDashboardWidgetList = components['schemas']['AvailableDashboardWidgetList']
export type DashboardWidgetCreate = components['schemas']['DashboardWidgetPlacementCreateRequest']

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/
const configurationLimit = 16 * 1024

export class InvalidDashboardResponseError extends Error {
  constructor() { super('Invalid Dashboard response') }
}

export class InvalidDashboardInputError extends Error {
  constructor() { super('Invalid Dashboard input') }
}

function object(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
function string(value: unknown): value is string { return typeof value === 'string' }
function positiveInteger(value: unknown): value is number { return typeof value === 'number' && Number.isInteger(value) && value > 0 }
function nonNegativeInteger(value: unknown): value is number { return typeof value === 'number' && Number.isInteger(value) && value >= 0 }
export function isDashboardPlacementID(value: string): boolean { return uuid.test(value) }

export function isDashboardConfiguration(value: unknown): value is Record<string, unknown> {
  if (!object(value)) return false
  try { return new TextEncoder().encode(JSON.stringify(value)).byteLength <= configurationLimit } catch { return false }
}

function isPlacement(value: unknown): value is DashboardWidgetPlacement {
  if (!object(value)) return false
  return string(value.placementId) && isDashboardPlacementID(value.placementId)
    && string(value.pluginId) && string(value.pluginVersion) && string(value.artifactDigest) && string(value.widgetId)
    && isDashboardConfiguration(value.configuration) && nonNegativeInteger(value.position)
    && typeof value.enabled === 'boolean' && positiveInteger(value.revision)
}
function isPlacementList(value: unknown): value is DashboardWidgetPlacementList {
  return object(value) && Array.isArray(value.widgets) && value.widgets.every(isPlacement)
}
function isAvailableWidget(value: unknown): value is AvailableDashboardWidget {
  return object(value) && string(value.pluginId) && string(value.pluginName) && string(value.pluginVersion)
    && string(value.artifactDigest) && string(value.widgetId) && string(value.widgetName) && string(value.description)
}
function isAvailableList(value: unknown): value is AvailableDashboardWidgetList {
  return object(value) && Array.isArray(value.widgets) && value.widgets.every(isAvailableWidget)
}
function json(method: string, body: unknown) {
  return { method, cache: 'no-store' as RequestCache, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }
}
function assertID(id: string) { if (!isDashboardPlacementID(id)) throw new InvalidDashboardInputError() }

export async function listDashboardWidgets(client: Pick<AuthService, 'request'> = useAuth()): Promise<DashboardWidgetPlacementList> {
  const value = await client.request<unknown>('/api/dashboard/widgets', { cache: 'no-store' })
  if (!isPlacementList(value)) throw new InvalidDashboardResponseError()
  return value
}
export async function listAvailableDashboardWidgets(client: Pick<AuthService, 'request'> = useAuth()): Promise<AvailableDashboardWidgetList> {
  const value = await client.request<unknown>('/api/dashboard/widgets/available', { cache: 'no-store' })
  if (!isAvailableList(value)) throw new InvalidDashboardResponseError()
  return value
}
export async function createDashboardWidget(input: DashboardWidgetCreate, client: Pick<AuthService, 'request'> = useAuth()): Promise<DashboardWidgetPlacement> {
  if (!isDashboardConfiguration(input.configuration)) throw new InvalidDashboardInputError()
  const value = await client.request<unknown>('/api/dashboard/widgets', json('POST', input))
  if (!isPlacement(value)) throw new InvalidDashboardResponseError()
  return value
}
export async function updateDashboardWidget(id: string, expectedRevision: number, configuration: Record<string, unknown>, enabled: boolean, client: Pick<AuthService, 'request'> = useAuth()): Promise<DashboardWidgetPlacement> {
  assertID(id)
  if (!positiveInteger(expectedRevision) || !isDashboardConfiguration(configuration)) throw new InvalidDashboardInputError()
  const value = await client.request<unknown>(`/api/dashboard/widgets/${encodeURIComponent(id)}`, json('PATCH', { expectedRevision, configuration, enabled }))
  if (!isPlacement(value)) throw new InvalidDashboardResponseError()
  return value
}
export async function moveDashboardWidget(id: string, direction: 'up' | 'down', expectedRevisions: Record<string, number>, client: Pick<AuthService, 'request'> = useAuth()): Promise<DashboardWidgetPlacementList> {
  assertID(id)
  if (!Object.entries(expectedRevisions).length || Object.entries(expectedRevisions).some(([key, revision]) => !isDashboardPlacementID(key) || !positiveInteger(revision))) throw new InvalidDashboardInputError()
  const value = await client.request<unknown>(`/api/dashboard/widgets/${encodeURIComponent(id)}/move-${direction}`, json('POST', { expectedRevisions }))
  if (!isPlacementList(value)) throw new InvalidDashboardResponseError()
  return value
}
export async function deleteDashboardWidget(id: string, expectedRevision: number, client: Pick<AuthService, 'request'> = useAuth()): Promise<void> {
  assertID(id)
  if (!positiveInteger(expectedRevision)) throw new InvalidDashboardInputError()
  await client.request<void>(`/api/dashboard/widgets/${encodeURIComponent(id)}`, { ...json('DELETE', { expectedRevision }), expectJSON: false })
}
// The runtime route deliberately sends no body. Every runtime coordinate and
// grant comes from the persisted placement on the server.
export async function launchDashboardWidget(id: string, client: Pick<APIClient, 'request'> = useAuth()): Promise<WidgetRuntimeLaunch> {
  assertID(id)
  const value = await client.request<unknown>(`/api/dashboard/widgets/${encodeURIComponent(id)}/widget-runtime`, { method: 'POST', cache: 'no-store' })
  if (!object(value)) throw new InvalidDashboardResponseError()
  return value as WidgetRuntimeLaunch
}

export function isDashboardConflict(error: unknown): boolean { return error instanceof APIProblemError && error.status === 409 }
export function isDashboardForbidden(error: unknown): boolean { return error instanceof APIProblemError && (error.status === 401 || error.status === 403) }
