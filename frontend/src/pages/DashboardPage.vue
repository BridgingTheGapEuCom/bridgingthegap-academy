<template>
  <BtgPageContainer as="section" class="dashboard-page" aria-labelledby="dashboard-title">
    <p class="sr-only" role="status" aria-live="polite">{{ announcement }}</p>
    <header class="dashboard-page__header">
      <div>
        <p class="dashboard-page__eyebrow">{{ t('dashboard.eyebrow') }}</p>
        <h1 id="dashboard-title" ref="heading" tabindex="-1">{{ t('dashboard.title') }}</h1>
        <p>{{ t('dashboard.description') }}</p>
      </div>
      <BtgButton v-if="canManage && !managing" type="button" variant="secondary" leading-icon="settings" @click="openManagement">{{ t('dashboard.manage') }}</BtgButton>
    </header>

    <section v-if="state.kind === 'loading'" class="dashboard-page__state" role="status"><p>{{ t('dashboard.loading') }}</p></section>
    <section v-else-if="state.kind === 'unavailable'" class="dashboard-page__state" role="alert" aria-labelledby="dashboard-unavailable-title">
      <h2 id="dashboard-unavailable-title">{{ t('dashboard.unavailableTitle') }}</h2>
      <p>{{ t('dashboard.unavailableDescription') }}</p>
      <BtgButton variant="secondary" @click="load">{{ t('common.actions.tryAgain') }}</BtgButton>
    </section>
    <template v-else>
      <section class="dashboard-page__widgets" aria-labelledby="dashboard-widgets-title">
        <h2 id="dashboard-widgets-title" class="sr-only">{{ t('dashboard.widgets') }}</h2>
        <div v-if="enabledWidgets.length === 0" class="dashboard-page__empty" role="status">
          <BtgIcon aria-hidden="true" class="dashboard-page__empty-icon" name="plus" />
          <div>
            <h2>{{ t('dashboard.emptyTitle') }}</h2>
            <p>{{ t('dashboard.emptyDescription') }}</p>
            <BtgButton v-if="canManage" type="button" leading-icon="plus" @click="openManagement">{{ t('dashboard.addWidget') }}</BtgButton>
          </div>
        </div>
        <ol v-else class="dashboard-page__widget-grid">
          <li v-for="placement in enabledWidgets" :key="runtimeKey(placement)">
            <BtgPanel as="article" class="dashboard-page__widget-card" :aria-labelledby="`dashboard-widget-${placement.placementId}`">
              <h2 :id="`dashboard-widget-${placement.placementId}`">{{ placementName(placement) }}</h2>
              <DashboardWidgetPlacementFrame :placement="placement" />
            </BtgPanel>
          </li>
        </ol>
      </section>

      <section v-if="managing && state.kind === 'ready'" class="dashboard-page__management" aria-labelledby="dashboard-management-title">
        <header class="dashboard-page__management-header">
          <div>
            <p class="dashboard-page__eyebrow">{{ t('dashboard.managementEyebrow') }}</p>
            <h2 id="dashboard-management-title" ref="managementHeading" tabindex="-1">{{ t('dashboard.managementTitle') }}</h2>
            <p>{{ t('dashboard.managementDescription') }}</p>
          </div>
          <BtgButton type="button" variant="secondary" @click="closeManagement">{{ t('dashboard.done') }}</BtgButton>
        </header>
        <p v-if="managementError" class="dashboard-page__error" role="alert">{{ managementError }}</p>

        <section class="dashboard-page__management-section" aria-labelledby="available-widgets-title">
          <h3 id="available-widgets-title">{{ t('dashboard.addWidget') }}</h3>
          <p v-if="availableLoading" role="status">{{ t('dashboard.availableLoading') }}</p>
          <p v-else-if="available.length === 0" class="dashboard-page__muted">{{ t('dashboard.noneAvailable') }}</p>
          <ul v-else class="dashboard-page__picker">
            <li v-for="widget in available" :key="widgetKey(widget)">
              <BtgPanel as="article" padding="compact">
                <div><h4>{{ widget.widgetName }}</h4><p>{{ widget.description }}</p><p class="dashboard-page__meta">{{ widget.pluginName }} · {{ widget.pluginVersion }}</p></div>
                <BtgButton type="button" variant="secondary" leading-icon="plus" :disabled="busy" :aria-label="t('dashboard.addNamed', { name: widget.widgetName })" @click="beginAdd(widget)">{{ t('dashboard.add') }}</BtgButton>
              </BtgPanel>
            </li>
          </ul>
        </section>

        <section v-if="configuringWidget" class="dashboard-page__configuration" aria-labelledby="add-configuration-title">
          <h3 id="add-configuration-title">{{ t('dashboard.configureNamed', { name: configuringWidget.widgetName }) }}</h3>
          <p>{{ t('dashboard.configureDescription') }}</p>
          <form @submit.prevent="createConfiguredPlacement">
            <WidgetConfigurationForm v-model="configurationDraft" :schema="configuringWidget.configuration!" :copy="widgetConfigurationCopy" :errors="configurationDraftErrors" />
            <div class="dashboard-page__actions"><BtgButton type="submit" leading-icon="plus" :disabled="busy || hasConfigurationErrors">{{ t('dashboard.addWidget') }}</BtgButton><BtgButton type="button" variant="secondary" :disabled="busy" @click="cancelConfiguration">{{ t('dashboard.cancel') }}</BtgButton></div>
          </form>
        </section>

        <section class="dashboard-page__management-section" aria-labelledby="installed-widgets-title">
          <h3 id="installed-widgets-title" ref="installedHeading" tabindex="-1">{{ t('dashboard.installed') }}</h3>
          <p v-if="state.widgets.length === 0" class="dashboard-page__muted">{{ t('dashboard.noneInstalled') }}</p>
          <ol v-else class="dashboard-page__placements" :aria-label="t('dashboard.configured')">
            <li v-for="(placement, index) in state.widgets" :key="placement.placementId">
              <BtgPanel as="article" class="dashboard-page__placement" padding="compact">
                <div class="dashboard-page__placement-summary">
                  <div><h4>{{ placementName(placement) }}</h4><p v-if="placementDescription(placement)">{{ placementDescription(placement) }}</p><p class="dashboard-page__meta">{{ placement.enabled ? t('dashboard.enabled') : t('dashboard.disabled') }} · {{ t('dashboard.position', { position: placement.position + 1 }) }}</p></div>
                  <BtgBadge>{{ placement.enabled ? t('dashboard.enabled') : t('dashboard.disabled') }}</BtgBadge>
                </div>
                <div class="dashboard-page__actions">
                  <BtgButton v-if="placementSchema(placement)?.fields.length" type="button" variant="secondary" leading-icon="settings" :disabled="busy" @click="beginEdit(placement)">{{ t('dashboard.configure') }}</BtgButton>
                  <BtgButton type="button" variant="secondary" leading-icon="previous" :disabled="busy || index === 0" :aria-label="t('dashboard.moveNamedUp', { name: placementName(placement) })" @click="move(placement, 'up')">{{ t('dashboard.moveUp') }}</BtgButton>
                  <BtgButton type="button" variant="secondary" trailing-icon="next" :disabled="busy || index === state.widgets.length - 1" :aria-label="t('dashboard.moveNamedDown', { name: placementName(placement) })" @click="move(placement, 'down')">{{ t('dashboard.moveDown') }}</BtgButton>
                  <BtgButton type="button" variant="secondary" :disabled="busy" @click="setEnabled(placement, !placement.enabled)">{{ placement.enabled ? t('dashboard.disable') : t('dashboard.enable') }}</BtgButton>
                  <template v-if="confirmingRemoval === placement.placementId">
                    <BtgButton type="button" variant="danger" leading-icon="delete" :disabled="busy" @click="remove(placement)">{{ t('dashboard.confirmRemove') }}</BtgButton>
                    <BtgButton type="button" variant="secondary" :disabled="busy" @click="confirmingRemoval = undefined">{{ t('dashboard.cancelRemove') }}</BtgButton>
                  </template>
                  <BtgButton v-else type="button" variant="danger-secondary" leading-icon="delete" :disabled="busy" @click="confirmingRemoval = placement.placementId">{{ t('dashboard.remove') }}</BtgButton>
                </div>
                <form v-if="editing === placement.placementId && placementSchema(placement)" class="dashboard-page__configuration" @submit.prevent="save(placement)">
                  <h5>{{ t('dashboard.configureNamed', { name: placementName(placement) }) }}</h5>
                  <WidgetConfigurationForm v-model="configurationDraft" :schema="placementSchema(placement)!" :copy="widgetConfigurationCopy" :errors="configurationDraftErrors" />
                  <div class="dashboard-page__actions"><BtgButton type="submit" :disabled="busy || hasConfigurationErrors">{{ t('dashboard.saveChanges') }}</BtgButton><BtgButton type="button" variant="secondary" :disabled="busy" @click="cancelConfiguration">{{ t('dashboard.cancel') }}</BtgButton></div>
                </form>
              </BtgPanel>
            </li>
          </ol>
        </section>
      </section>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  createDashboardWidget, deleteDashboardWidget, isDashboardConflict, isDashboardForbidden,
  listAvailableDashboardWidgets, listDashboardWidgets, moveDashboardWidget, updateDashboardWidget,
  type AvailableDashboardWidget, type DashboardWidgetPlacement,
} from '../dashboard/dashboard'
import { configurationErrors, configurationInitial, type WidgetConfiguration, type WidgetConfigurationSchema } from '../plugins/configuration'
import BtgButton from '../components/BtgButton.vue'
import BtgBadge from '../components/BtgBadge.vue'
import BtgIcon from '../components/BtgIcon.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPanel from '../components/BtgPanel.vue'
import DashboardWidgetPlacementFrame from '../components/DashboardWidgetPlacement.vue'
import WidgetConfigurationForm from '../components/WidgetConfigurationForm.vue'

type State = { kind: 'loading' } | { kind: 'ready'; widgets: DashboardWidgetPlacement[] } | { kind: 'unavailable' }
const { t } = useI18n()
const widgetConfigurationCopy = computed(() => ({ selectOption: t('dashboard.selectOption'), enabled: t('dashboard.enabled'), disabled: t('dashboard.disabled') }))
const state = ref<State>({ kind: 'loading' })
const heading = ref<HTMLElement>()
const managementHeading = ref<HTMLElement>()
const installedHeading = ref<HTMLElement>()
const announcement = ref('')
const managementError = ref('')
const managing = ref(false)
const availableLoading = ref(false)
const available = ref<AvailableDashboardWidget[]>([])
const canManage = ref(false)
const configuringWidget = ref<AvailableDashboardWidget>()
const editing = ref<string>()
const confirmingRemoval = ref<string>()
const configurationDraft = ref<WidgetConfiguration>({})
const busy = ref(false)
let active = true
let generation = 0

const enabledWidgets = computed(() => state.value.kind === 'ready' ? state.value.widgets.filter((placement) => placement.enabled) : [])
const activeSchema = computed<WidgetConfigurationSchema | null>(() => configuringWidget.value?.configuration ?? (state.value.kind === 'ready' && editing.value ? placementSchema(state.value.widgets.find((item) => item.placementId === editing.value)) : null))
const configurationDraftErrors = computed(() => configurationErrors(activeSchema.value, configurationDraft.value))
const hasConfigurationErrors = computed(() => Object.keys(configurationDraftErrors.value).length > 0)

function widgetKey(widget: AvailableDashboardWidget): string { return [widget.pluginId, widget.pluginVersion, widget.artifactDigest, widget.widgetId].join(':') }
function runtimeKey(placement: DashboardWidgetPlacement): string { return `${placement.placementId}:${placement.revision}:${placement.enabled}` }
function metadata(placement?: DashboardWidgetPlacement): AvailableDashboardWidget | undefined { return placement && available.value.find((widget) => widget.pluginId === placement.pluginId && widget.pluginVersion === placement.pluginVersion && widget.artifactDigest === placement.artifactDigest && widget.widgetId === placement.widgetId) }
function placementName(placement: DashboardWidgetPlacement): string { return metadata(placement)?.widgetName ?? placement.widgetId }
function placementDescription(placement: DashboardWidgetPlacement): string { return metadata(placement)?.description ?? '' }
function placementSchema(placement?: DashboardWidgetPlacement): WidgetConfigurationSchema | null { return metadata(placement)?.configuration ?? null }
function sorted(widgets: DashboardWidgetPlacement[]): DashboardWidgetPlacement[] { return [...widgets].sort((a, b) => a.position - b.position) }
function setWidgets(widgets: DashboardWidgetPlacement[]) { if (state.value.kind === 'ready') state.value = { kind: 'ready', widgets: sorted(widgets) } }
function replace(placement: DashboardWidgetPlacement) { if (state.value.kind === 'ready') setWidgets(state.value.widgets.map((current) => current.placementId === placement.placementId ? placement : current)) }
function revisions(): Record<string, number> { return state.value.kind === 'ready' ? Object.fromEntries(state.value.widgets.map((placement) => [placement.placementId, placement.revision])) : {} }

async function load(clearError = true) {
  const current = ++generation
  state.value = { kind: 'loading' }
  if (clearError) managementError.value = ''
  try {
    const response = await listDashboardWidgets()
    if (!active || current !== generation) return
    state.value = { kind: 'ready', widgets: sorted(response.widgets) }
    announcement.value = t('dashboard.loaded')
    void loadAvailable()
  } catch { if (active && current === generation) state.value = { kind: 'unavailable' } }
}
async function loadAvailable() {
  availableLoading.value = true
  try {
    const response = await listAvailableDashboardWidgets()
    if (active) { available.value = response.widgets; canManage.value = true }
  } catch (error) {
    if (active) { canManage.value = false; if (managing.value && !isDashboardForbidden(error)) managementError.value = t('dashboard.availableUnavailable') }
  } finally { if (active) availableLoading.value = false }
}
async function openManagement() { managing.value = true; await nextTick(); managementHeading.value?.focus() }
function closeManagement() { managing.value = false; cancelConfiguration(); confirmingRemoval.value = undefined; managementError.value = ''; void nextTick(() => heading.value?.focus()) }
function cancelConfiguration() { configuringWidget.value = undefined; editing.value = undefined; configurationDraft.value = {} }
function beginAdd(widget: AvailableDashboardWidget) {
  managementError.value = ''
  if (!widget.configuration?.fields.length) { void createPlacement(widget, {}); return }
  editing.value = undefined; configuringWidget.value = widget; configurationDraft.value = configurationInitial(widget.configuration)
}
function beginEdit(placement: DashboardWidgetPlacement) {
  const schema = placementSchema(placement)
  if (!schema) return
  configuringWidget.value = undefined; editing.value = placement.placementId; configurationDraft.value = configurationInitial(schema, placement.configuration)
}
async function conflict(message: string) { managementError.value = message; announcement.value = message; const draft = configurationDraft.value; await load(false); configurationDraft.value = draft; busy.value = false; await nextTick(); managementHeading.value?.focus() }
async function createConfiguredPlacement() { if (configuringWidget.value && !hasConfigurationErrors.value) await createPlacement(configuringWidget.value, configurationDraft.value) }
async function createPlacement(widget: AvailableDashboardWidget, configuration: WidgetConfiguration) {
  if (busy.value || state.value.kind !== 'ready') return
  const current = ++generation; busy.value = true; managementError.value = ''
  try {
    const result = await createDashboardWidget({ pluginId: widget.pluginId, pluginVersion: widget.pluginVersion, artifactDigest: widget.artifactDigest, widgetId: widget.widgetId, configuration })
    if (!active || current !== generation) return
    setWidgets([...state.value.widgets, result]); cancelConfiguration(); announcement.value = t('dashboard.added', { name: widget.widgetName }); await nextTick(); installedHeading.value?.focus()
  } catch (error) { if (active && current === generation) { if (isDashboardConflict(error)) await conflict(t('dashboard.configurationChanged')); else managementError.value = t('dashboard.widgetAddFailed') } } finally { if (active && current === generation) busy.value = false }
}
async function save(placement: DashboardWidgetPlacement) {
  if (busy.value || hasConfigurationErrors.value) return
  const current = ++generation; busy.value = true; managementError.value = ''
  try {
    const result = await updateDashboardWidget(placement.placementId, placement.revision, configurationDraft.value, placement.enabled)
    if (!active || current !== generation) return
    replace(result); cancelConfiguration(); announcement.value = t('dashboard.configurationSaved'); await nextTick(); managementHeading.value?.focus()
  } catch (error) { if (active && current === generation) { if (isDashboardConflict(error)) await conflict(t('dashboard.configurationConflict')); else managementError.value = t('dashboard.widgetSaveFailed') } } finally { if (active && current === generation) busy.value = false }
}
async function setEnabled(placement: DashboardWidgetPlacement, enabled: boolean) {
  if (busy.value) return
  const current = ++generation; busy.value = true; managementError.value = ''
  try { const result = await updateDashboardWidget(placement.placementId, placement.revision, placement.configuration, enabled); if (!active || current !== generation) return; replace(result); announcement.value = enabled ? t('dashboard.enabledAnnouncement') : t('dashboard.disabledAnnouncement') }
  catch (error) { if (active && current === generation) { if (isDashboardConflict(error)) await conflict(t('dashboard.configurationChanged')); else managementError.value = t('dashboard.updateFailed') } } finally { if (active && current === generation) busy.value = false }
}
async function move(placement: DashboardWidgetPlacement, direction: 'up' | 'down') {
  if (busy.value) return
  const current = ++generation; busy.value = true; managementError.value = ''
  try { const result = await moveDashboardWidget(placement.placementId, direction, revisions()); if (!active || current !== generation) return; setWidgets(result.widgets); announcement.value = t(direction === 'up' ? 'dashboard.movedUp' : 'dashboard.movedDown') }
  catch (error) { if (active && current === generation) { if (isDashboardConflict(error)) await conflict(t('dashboard.orderConflict')); else managementError.value = t('dashboard.updateFailed') } } finally { if (active && current === generation) busy.value = false }
}
async function remove(placement: DashboardWidgetPlacement) {
  if (busy.value) return
  const current = ++generation; busy.value = true; managementError.value = ''
  try { await deleteDashboardWidget(placement.placementId, placement.revision); if (!active || current !== generation) return; setWidgets(state.value.kind === 'ready' ? state.value.widgets.filter((item) => item.placementId !== placement.placementId) : []); cancelConfiguration(); confirmingRemoval.value = undefined; announcement.value = t('dashboard.removed'); await nextTick(); installedHeading.value?.focus() }
  catch (error) { if (active && current === generation) { if (isDashboardConflict(error)) await conflict(t('dashboard.configurationChanged')); else managementError.value = t('dashboard.removeFailed') } } finally { if (active && current === generation) busy.value = false }
}

void load()
onBeforeUnmount(() => { active = false; generation += 1 })
</script>

<style scoped>
.dashboard-page { --btg-color-link: var(--btg-home-text); --btg-color-link-hover: var(--btg-home-text-secondary); --btg-color-focus: var(--btg-home-focus); --btg-color-danger: var(--btg-home-interactive); --btg-color-danger-hover: var(--btg-home-interactive-active); display: grid; min-width: 0; gap: clamp(var(--btg-space-6), 5vw, var(--btg-space-8)); color: var(--btg-home-text); overflow-wrap: anywhere; }
.dashboard-page * { min-width: 0; }
.dashboard-page__header, .dashboard-page__management-header, .dashboard-page__placement-summary, .dashboard-page__picker article { display: flex; flex-wrap: wrap; align-items: start; justify-content: space-between; gap: var(--btg-space-4); }
.dashboard-page__header > div, .dashboard-page__management-header > div { display: grid; gap: var(--btg-space-2); }
.dashboard-page__header p, .dashboard-page__management-header p, .dashboard-page__muted, .dashboard-page__placement p, .dashboard-page__picker p { margin: 0; color: var(--btg-home-text-secondary); }
.dashboard-page__eyebrow { color: var(--btg-home-text-secondary) !important; font-size: var(--btg-font-size-small); font-weight: var(--btg-font-weight-strong); letter-spacing: .09em; text-transform: uppercase; }
.dashboard-page__widget-grid, .dashboard-page__picker, .dashboard-page__placements { display: grid; margin: 0; padding: 0; list-style: none; }
.dashboard-page__widget-grid { grid-template-columns: repeat(auto-fit, minmax(min(100%, 22rem), 1fr)); gap: var(--btg-space-5); }
.dashboard-page__widget-card { display: grid; min-height: 14rem; gap: var(--btg-space-4); }
.dashboard-page__widget-card h2 { font-size: var(--btg-font-size-h3); }
.dashboard-page__empty { display: flex; align-items: center; gap: var(--btg-space-5); border: 1px solid var(--btg-home-border); background: var(--btg-home-surface-muted); padding: clamp(var(--btg-space-5), 5vw, var(--btg-space-7)); }
.dashboard-page__empty > div:last-child { display: grid; gap: var(--btg-space-3); }
.dashboard-page__empty p { margin: 0; color: var(--btg-home-text-secondary); }
.dashboard-page__empty-icon { display: grid; width: 3rem; height: 3rem; flex: 0 0 auto; place-items: center; border-radius: 50%; background: var(--btg-home-icon-surface); font-size: 1.5rem; }
.dashboard-page__management { display: grid; gap: var(--btg-space-6); border-top: 1px solid var(--btg-home-border); background: var(--btg-home-surface-muted); padding: clamp(var(--btg-space-5), 5vw, var(--btg-space-7)); }
.dashboard-page__management-section { display: grid; gap: var(--btg-space-4); }
.dashboard-page__picker { grid-template-columns: repeat(auto-fit, minmax(min(100%, 18rem), 1fr)); gap: var(--btg-space-4); }
.dashboard-page__picker article > div { display: grid; gap: var(--btg-space-2); }
.dashboard-page__meta { font-size: var(--btg-font-size-small); }
.dashboard-page__placements { gap: var(--btg-space-3); }
.dashboard-page__placement { display: grid; gap: var(--btg-space-4); }
.dashboard-page__placement-summary > div { display: grid; gap: var(--btg-space-2); }
.dashboard-page__status-label { border: 1px solid var(--btg-home-border); border-radius: 999px; background: var(--btg-home-surface-muted); padding: var(--btg-space-1) var(--btg-space-3); font-size: var(--btg-font-size-small); font-weight: var(--btg-font-weight-strong); }
.dashboard-page__configuration { display: grid; gap: var(--btg-space-4); border-inline-start: .25rem solid var(--btg-home-border-strong); background: var(--btg-home-background); padding: var(--btg-space-5); }
.dashboard-page__configuration form, .dashboard-page__configuration:not(form) > form { display: grid; gap: var(--btg-space-4); }
.dashboard-page__actions { display: flex; flex-wrap: wrap; gap: var(--btg-space-2); }
.dashboard-page__error { border-inline-start: .25rem solid var(--btg-color-danger); background: var(--btg-home-background); padding: var(--btg-space-3) var(--btg-space-4); }
@media (max-width: 32rem) { .dashboard-page__header, .dashboard-page__management-header, .dashboard-page__empty { align-items: stretch; flex-direction: column; } .dashboard-page__actions > * { flex: 1 1 9rem; } }
</style>
