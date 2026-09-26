<template>
  <BtgPageContainer as="section" class="dashboard-page" width="application" aria-labelledby="dashboard-title">
    <p class="sr-only" role="status" aria-live="polite">{{ announcement }}</p>
    <header class="dashboard-page__header">
      <p class="dashboard-page__eyebrow">Bridging the Gap Academy</p>
      <h1 id="dashboard-title" ref="heading" tabindex="-1">Dashboard</h1>
      <p>Installed Dashboard widgets appear here in their configured order.</p>
    </header>

    <section v-if="state.kind === 'loading'" role="status" aria-label="Loading Dashboard"><p>Loading Dashboard…</p></section>
    <section v-else-if="state.kind === 'unavailable'" role="alert" aria-labelledby="dashboard-unavailable-title">
      <h2 id="dashboard-unavailable-title">Dashboard unavailable</h2>
      <p>We couldn’t load the Dashboard right now. Please try again.</p>
      <BtgButton variant="secondary" @click="load">Try again</BtgButton>
    </section>
    <section v-else-if="state.kind === 'forbidden'" aria-labelledby="dashboard-empty-title">
      <h2 id="dashboard-empty-title">Dashboard</h2>
      <p>There are no Dashboard widgets available to you.</p>
    </section>
    <template v-else>
      <section class="dashboard-page__widgets" aria-labelledby="dashboard-widgets-title">
        <h2 id="dashboard-widgets-title">Dashboard widgets</h2>
        <p v-if="enabledWidgets.length === 0" role="status">There are no enabled Dashboard widgets.</p>
        <ol v-else class="dashboard-page__widget-list">
          <li v-for="placement in enabledWidgets" :key="runtimeKey(placement)">
            <DashboardWidgetPlacementFrame :placement="placement" />
          </li>
        </ol>
      </section>

      <section class="dashboard-page__management" aria-labelledby="dashboard-management-title">
        <h2 id="dashboard-management-title">Configure Dashboard widgets</h2>
        <p>Changes apply to this Academy Dashboard. A widget release stays pinned until you remove this placement.</p>
        <p v-if="managementError" class="dashboard-page__error" role="alert">{{ managementError }}</p>

        <details :open="adding" @toggle="toggleAdd">
          <summary>Add dashboard widget</summary>
          <form v-if="adding" class="dashboard-page__form" @submit.prevent="createPlacement">
            <BtgFormField label="Dashboard widget" :error="available.length === 0 ? 'No Dashboard widgets are currently available.' : undefined" required v-slot="{ controlId, describedBy, invalid }">
              <select :id="controlId" v-model="selectedWidget" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" :disabled="busy || availableLoading">
                <option value="" disabled>Select a widget</option>
                <option v-for="widget in available" :key="widgetKey(widget)" :value="widgetKey(widget)">{{ widget.widgetName }} · {{ widget.pluginName }} {{ widget.pluginVersion }}</option>
              </select>
            </BtgFormField>
            <BtgFormField label="Configuration (JSON)" :error="addConfigurationError" required v-slot="{ controlId, describedBy, invalid }">
              <textarea :id="controlId" v-model="addConfiguration" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" maxlength="16384" spellcheck="false" :disabled="busy" />
            </BtgFormField>
            <p v-if="availableLoading" role="status">Loading available widgets…</p>
            <div class="dashboard-page__actions"><BtgButton type="submit" :disabled="busy || !selectedWidget || !!addConfigurationError">Add dashboard widget</BtgButton><BtgButton type="button" variant="secondary" :disabled="busy" @click="closeAdd">Cancel</BtgButton></div>
          </form>
        </details>

        <ol class="dashboard-page__placements" aria-label="Configured Dashboard widgets">
          <li v-for="(placement, index) in state.widgets" :key="placement.placementId">
            <article class="dashboard-page__placement">
              <h3>{{ placement.widgetId }}</h3>
              <p>Release {{ placement.pluginVersion }} · Position {{ placement.position + 1 }} · {{ placement.enabled ? 'Enabled' : 'Disabled' }}</p>
              <p v-if="editing === placement.placementId" class="dashboard-page__note">The pinned plugin release cannot be changed. Remove this placement and add another widget to switch releases.</p>
              <div class="dashboard-page__actions">
                <BtgButton type="button" variant="secondary" :disabled="busy" @click="edit(placement)">{{ editing === placement.placementId ? 'Close configuration' : 'Configure placement' }}</BtgButton>
                <BtgButton type="button" variant="secondary" :disabled="busy || index === 0" :aria-label="`Move ${placement.widgetId} up`" @click="move(placement, 'up')">Move up</BtgButton>
                <BtgButton type="button" variant="secondary" :disabled="busy || index === state.widgets.length - 1" :aria-label="`Move ${placement.widgetId} down`" @click="move(placement, 'down')">Move down</BtgButton>
                <BtgButton type="button" variant="secondary" :disabled="busy" @click="setEnabled(placement, !placement.enabled)">{{ placement.enabled ? 'Disable placement' : 'Enable placement' }}</BtgButton>
                <BtgButton type="button" variant="destructive" :disabled="busy" @click="remove(placement)">Remove placement</BtgButton>
              </div>
              <form v-if="editing === placement.placementId" class="dashboard-page__form" @submit.prevent="save(placement)">
                <BtgFormField label="Configuration (JSON)" :error="editConfigurationError" required v-slot="{ controlId, describedBy, invalid }">
                  <textarea :id="controlId" v-model="editConfiguration" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" maxlength="16384" spellcheck="false" :disabled="busy" />
                </BtgFormField>
                <div class="dashboard-page__actions"><BtgButton type="submit" :disabled="busy || !!editConfigurationError">Save configuration</BtgButton><BtgButton type="button" variant="secondary" :disabled="busy" @click="editing = undefined">Cancel</BtgButton></div>
              </form>
            </article>
          </li>
        </ol>
      </section>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { APIProblemError } from '../api/client'
import {
  createDashboardWidget, deleteDashboardWidget, isDashboardConfiguration, isDashboardConflict, isDashboardForbidden,
  listAvailableDashboardWidgets, listDashboardWidgets, moveDashboardWidget, updateDashboardWidget,
  type AvailableDashboardWidget, type DashboardWidgetPlacement,
} from '../dashboard/dashboard'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import DashboardWidgetPlacementFrame from '../components/DashboardWidgetPlacement.vue'

type State = { kind: 'loading' } | { kind: 'ready'; widgets: DashboardWidgetPlacement[] } | { kind: 'forbidden' } | { kind: 'unavailable' }
const state = ref<State>({ kind: 'loading' })
const heading = ref<HTMLElement>()
const announcement = ref('')
const managementError = ref('')
const adding = ref(false)
const availableLoading = ref(false)
const available = ref<AvailableDashboardWidget[]>([])
const selectedWidget = ref('')
const addConfiguration = ref('{}')
const editing = ref<string>()
const editConfiguration = ref('{}')
const busy = ref(false)
let active = true
let generation = 0

const enabledWidgets = computed(() => state.value.kind === 'ready' ? state.value.widgets.filter((placement) => placement.enabled) : [])
const addConfigurationError = computed(() => configurationError(addConfiguration.value))
const editConfigurationError = computed(() => configurationError(editConfiguration.value))

function widgetKey(widget: AvailableDashboardWidget): string { return [widget.pluginId, widget.pluginVersion, widget.artifactDigest, widget.widgetId].join(':') }
function runtimeKey(placement: DashboardWidgetPlacement): string { return `${placement.placementId}:${placement.revision}:${placement.enabled}` }
function parseConfiguration(value: string): Record<string, unknown> | undefined {
  try { const parsed: unknown = JSON.parse(value); return isDashboardConfiguration(parsed) ? parsed : undefined } catch { return undefined }
}
function configurationError(value: string): string | undefined {
  return parseConfiguration(value) ? undefined : 'Enter a JSON object no larger than 16 KiB.'
}
function sorted(widgets: DashboardWidgetPlacement[]): DashboardWidgetPlacement[] { return [...widgets].sort((a, b) => a.position - b.position) }
function setWidgets(widgets: DashboardWidgetPlacement[]) { if (state.value.kind === 'ready') state.value = { kind: 'ready', widgets: sorted(widgets) } }
function replace(placement: DashboardWidgetPlacement) {
  if (state.value.kind !== 'ready') return
  setWidgets(state.value.widgets.map((current) => current.placementId === placement.placementId ? placement : current))
}
function revisions(): Record<string, number> {
  if (state.value.kind !== 'ready') return {}
  return Object.fromEntries(state.value.widgets.map((placement) => [placement.placementId, placement.revision]))
}

async function load(clearError = true) {
  const current = ++generation
  state.value = { kind: 'loading' }
  if (clearError) managementError.value = ''
  try {
    const response = await listDashboardWidgets()
    if (!active || current !== generation) return
    state.value = { kind: 'ready', widgets: sorted(response.widgets) }
    announcement.value = 'Dashboard loaded.'
  } catch (error) {
    if (!active || current !== generation) return
    state.value = isDashboardForbidden(error) ? { kind: 'forbidden' } : { kind: 'unavailable' }
  }
}

async function toggleAdd(event: Event) {
  const open = (event.target as HTMLDetailsElement).open
  adding.value = open
  if (!open || available.value.length || availableLoading.value) return
  availableLoading.value = true
  try {
    const response = await listAvailableDashboardWidgets()
    if (active && adding.value) available.value = response.widgets
  } catch {
    if (active) managementError.value = 'Available Dashboard widgets could not be loaded. Try again.'
  } finally { if (active) availableLoading.value = false }
}
function closeAdd() { adding.value = false; selectedWidget.value = ''; addConfiguration.value = '{}' }
function edit(placement: DashboardWidgetPlacement) {
  if (editing.value === placement.placementId) { editing.value = undefined; return }
  editing.value = placement.placementId
  editConfiguration.value = JSON.stringify(placement.configuration, null, 2)
  managementError.value = ''
}
async function conflict(message: string) {
  managementError.value = message
  announcement.value = message
  await load(false)
  await nextTick()
  if (active) busy.value = false
  heading.value?.focus()
}
async function createPlacement() {
  const widget = available.value.find((candidate) => widgetKey(candidate) === selectedWidget.value)
  const configuration = parseConfiguration(addConfiguration.value)
  if (!widget || !configuration || busy.value || state.value.kind !== 'ready') return
  const current = ++generation; busy.value = true; managementError.value = ''
  try {
    const result = await createDashboardWidget({ pluginId: widget.pluginId, pluginVersion: widget.pluginVersion, artifactDigest: widget.artifactDigest, widgetId: widget.widgetId, configuration })
    if (!active || current !== generation) return
    setWidgets([...state.value.widgets, result])
    closeAdd(); announcement.value = 'Dashboard widget added.'
    await nextTick(); heading.value?.focus()
  } catch (error) { if (active && current === generation) { if (isDashboardConflict(error)) await conflict('Dashboard configuration changed. The latest Dashboard has been loaded.'); else managementError.value = 'The Dashboard widget could not be added. Try again.' } } finally { if (active && current === generation) busy.value = false }
}
async function save(placement: DashboardWidgetPlacement) {
  const configuration = parseConfiguration(editConfiguration.value)
  if (!configuration || busy.value) return
  const current = ++generation; busy.value = true; managementError.value = ''
  try {
    const result = await updateDashboardWidget(placement.placementId, placement.revision, configuration, placement.enabled)
    if (!active || current !== generation) return
    replace(result); editing.value = undefined; announcement.value = 'Dashboard widget configuration saved.'
  } catch (error) { if (active && current === generation) { if (isDashboardConflict(error)) await conflict('Dashboard configuration changed. The latest Dashboard has been loaded.'); else managementError.value = 'The Dashboard widget could not be saved. Try again.' } } finally { if (active && current === generation) busy.value = false }
}
async function setEnabled(placement: DashboardWidgetPlacement, enabled: boolean) {
  if (busy.value) return
  const current = ++generation; busy.value = true; managementError.value = ''
  try {
    const result = await updateDashboardWidget(placement.placementId, placement.revision, placement.configuration, enabled)
    if (!active || current !== generation) return
    replace(result); announcement.value = enabled ? 'Dashboard widget enabled.' : 'Dashboard widget disabled.'
  } catch (error) { if (active && current === generation) { if (isDashboardConflict(error)) await conflict('Dashboard configuration changed. The latest Dashboard has been loaded.'); else managementError.value = 'The Dashboard widget could not be updated. Try again.' } } finally { if (active && current === generation) busy.value = false }
}
async function move(placement: DashboardWidgetPlacement, direction: 'up' | 'down') {
  if (busy.value) return
  const current = ++generation; busy.value = true; managementError.value = ''
  try {
    const result = await moveDashboardWidget(placement.placementId, direction, revisions())
    if (!active || current !== generation) return
    setWidgets(result.widgets); announcement.value = direction === 'up' ? 'Dashboard widget moved up.' : 'Dashboard widget moved down.'
  } catch (error) { if (active && current === generation) { if (isDashboardConflict(error)) await conflict('Dashboard order changed. The latest Dashboard has been loaded.'); else managementError.value = 'The Dashboard order could not be updated. Try again.' } } finally { if (active && current === generation) busy.value = false }
}
async function remove(placement: DashboardWidgetPlacement) {
  if (busy.value) return
  const current = ++generation; busy.value = true; managementError.value = ''
  try {
    await deleteDashboardWidget(placement.placementId, placement.revision)
    if (!active || current !== generation) return
    setWidgets(state.value.kind === 'ready' ? state.value.widgets.filter((item) => item.placementId !== placement.placementId) : [])
    if (editing.value === placement.placementId) editing.value = undefined
    announcement.value = 'Dashboard widget removed.'
    await nextTick(); heading.value?.focus()
  } catch (error) { if (active && current === generation) { if (isDashboardConflict(error)) await conflict('Dashboard configuration changed. The latest Dashboard has been loaded.'); else managementError.value = 'The Dashboard widget could not be removed. Try again.' } } finally { if (active && current === generation) busy.value = false }
}

void load()
onBeforeUnmount(() => { active = false; generation += 1 })
</script>

<style scoped>
.dashboard-page__header, .dashboard-page__management { max-width: 72rem; }
.dashboard-page__widget-list, .dashboard-page__placements { display: grid; gap: 1rem; padding: 0; list-style: none; }
.dashboard-page__placement { border: 1px solid var(--color-border, #b8b8b8); border-radius: .5rem; padding: 1rem; }
.dashboard-page__actions { display: flex; flex-wrap: wrap; gap: .5rem; margin-top: 1rem; }
.dashboard-page__form { display: grid; gap: 1rem; margin-top: 1rem; max-width: 48rem; }
.dashboard-page__form textarea { box-sizing: border-box; width: 100%; min-height: 10rem; font-family: ui-monospace, monospace; }
.dashboard-page__error { color: var(--color-danger, #9b1c1c); }
.dashboard-page__note { font-size: .95rem; }
@media (max-width: 20rem) { .dashboard-page__actions > * { width: 100%; } }
</style>
