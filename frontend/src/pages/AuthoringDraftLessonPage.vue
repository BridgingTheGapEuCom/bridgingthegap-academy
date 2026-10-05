<template>
  <section class="authoring-section authoring-lesson-editor" aria-labelledby="authoring-lesson-title">
    <div v-if="state.kind === 'loading'" class="authoring-lesson-editor__state" role="status">
      <h2 id="authoring-lesson-title">Loading Lesson…</h2>
      <p>One moment while we load this Lesson.</p>
    </div>

    <div v-else-if="state.kind === 'unavailable'" class="authoring-lesson-editor__state">
      <h2 id="authoring-lesson-title">Lesson unavailable</h2>
      <p>We couldn’t load this Lesson right now.</p>
      <BtgButton variant="secondary" @click="load">Try again</BtgButton>
    </div>

    <template v-else-if="state.kind === 'ready'">
      <AuthoringLessonHeader :title="state.lesson.title" :back-to="backToStructure" :tabs="lessonTabs" />

      <template v-if="currentSection === 'details' || legacyCombined">
      <AuthoringPageTitle id="authoring-lesson-details-title" :title="t('authoring.details.title')" :description="t('authoring.details.description')" focusable />
      <form class="authoring-lesson-editor__form" :aria-busy="saving" novalidate @submit.prevent="save">
        <p v-if="formError" class="authoring-lesson-editor__error" role="alert">{{ formError }}</p>
        <p v-if="saveMessage" class="authoring-lesson-editor__status" role="status">{{ saveMessage }}</p>
        <div v-if="conflict" class="authoring-lesson-editor__conflict" role="status">
          <p>This Lesson changed elsewhere. Your edits are still here, but you need to reload the latest Lesson before saving again.</p>
          <BtgButton variant="secondary" :disabled="reloading" @click="reloadLatest">{{ reloading ? 'Reloading…' : 'Reload latest Lesson' }}</BtgButton>
        </div>

        <AuthoringSection heading-id="authoring-lesson-basic-information-title" :title="t('authoring.details.basicInformation')" :description="t('authoring.details.basicInformationDescription')">
            <BtgFormField :label="t('authoring.details.lessonTitle')" required :error="errors.title" v-slot="{ controlId, describedBy, invalid }">
              <BtgTextInput :id="controlId" v-model="form.title" :disabled="saving || reloading" :aria-describedby="describedBy" :invalid="invalid" required />
            </BtgFormField>
            <BtgFormField :label="t('authoring.details.lessonDescription')" required :error="errors.description" v-slot="{ controlId, describedBy, invalid }">
              <textarea :id="controlId" v-model="form.description" :disabled="saving || reloading" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" required />
            </BtgFormField>
        </AuthoringSection>

        <AuthoringSection heading-id="authoring-lesson-objectives-title" :title="t('authoring.details.objectives')" :description="t('authoring.details.objectivesDescription')">
          <RepeatableObjectivesEditor v-model="form.objectives" :disabled="saving || reloading" :error="errors.objectives" :label="t('authoring.details.objectives')" :add-label="t('authoring.details.addObjective')" :reorder-label="(position) => t('authoring.details.reorderObjective', { position })" :remove-label-for="(position) => t('authoring.details.removeObjective', { position })" :remove-text="t('authoring.details.remove')" />
        </AuthoringSection>

        <AuthoringSection heading-id="authoring-lesson-timing-title" :title="t('authoring.details.timing')" :description="t('authoring.details.timingDescription')">
            <BtgFormField :label="t('authoring.details.estimatedDuration')" :error="errors.estimatedDurationMinutes" v-slot="{ controlId, describedBy, invalid }">
              <div class="authoring-lesson-editor__duration"><BtgTextInput :id="controlId" v-model="form.estimatedDurationMinutes" :disabled="saving || reloading" :aria-describedby="describedBy" :invalid="invalid" type="number" inputmode="numeric" min="1" max="1440" step="1" /><span>{{ t('authoring.details.minutes') }}</span></div>
            </BtgFormField>
        </AuthoringSection>

        <AuthoringDirtyActionBar :show="dirty" :busy="saving" :disabled="saving || reloading" :save-disabled="conflict" @discard="discardChanges" />
      </form>
      </template>
      <template v-if="currentSection === 'content' || legacyCombined">
        <AuthoringPageTitle id="authoring-lesson-content-title" :title="t('authoring.content.title')" :description="t('authoring.content.description')" focusable />
        <AuthoringLessonContentEditor :draft-id="draft.id" :lesson="state.lesson" @saved="applyContent" @replace-lesson="replaceLesson" @preview="openPreview" @unavailable="markDraftUnavailable" />
      </template>
      <template v-if="currentSection === 'prerequisites' || legacyCombined">
        <AuthoringPageTitle id="authoring-lesson-prerequisites-title" :title="t('authoring.prerequisites.title')" :description="t('authoring.prerequisites.description')" focusable />
      <AuthoringLessonPrerequisitesEditor
        :draft-id="draft.id"
        :lesson="state.lesson"
        @saved="applyPrerequisites"
        @replace-lesson="replaceLesson"
        @unavailable="markDraftUnavailable"
      />
      </template>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { APIProblemError } from '../api/client'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { authoringDraftPath, getAuthoringLesson, InvalidAuthoringDraftIDError, updateAuthoringLesson, type AuthoringLessonDetail, type AuthoringLessonMetadataPatch } from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgTextInput from '../components/BtgTextInput.vue'
import AuthoringLessonPrerequisitesEditor from '../components/AuthoringLessonPrerequisitesEditor.vue'
import AuthoringLessonContentEditor from '../components/AuthoringLessonContentEditor.vue'
import AuthoringDirtyActionBar from '../components/AuthoringDirtyActionBar.vue'
import AuthoringLessonHeader from '../components/AuthoringLessonHeader.vue'
import AuthoringPageTitle from '../components/AuthoringPageTitle.vue'
import AuthoringSection from '../components/AuthoringSection.vue'
import RepeatableObjectivesEditor from '../components/RepeatableObjectivesEditor.vue'

type Form = { title: string; description: string; objectives: string[]; estimatedDurationMinutes: string }
type State = { kind: 'loading' } | { kind: 'ready'; lesson: AuthoringLessonDetail } | { kind: 'unavailable' }

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const { draft, replaceDraft, markDraftUnavailable } = useAuthoringDraftContext()
const state = ref<State>({ kind: 'loading' })
const original = ref<Form>()
const form = reactive<Form>({ title: '', description: '', objectives: [''], estimatedDurationMinutes: '' })
const errors = reactive<Record<string, string | undefined>>({})
const formError = ref<string>()
const saveMessage = ref<string>()
const saving = ref(false)
const reloading = ref(false)
const conflict = ref(false)
const dirty = computed(() => original.value !== undefined && JSON.stringify(normalized(form)) !== JSON.stringify(normalized(original.value)))
const sections = [{ id: 'details', label: 'Details' }, { id: 'content', label: 'Content' }, { id: 'prerequisites', label: 'Prerequisites' }] as const
type Section = typeof sections[number]['id']
const currentSection = computed<Section>(() => route.name === 'authoring-draft-lesson-content' ? 'content' : route.name === 'authoring-draft-lesson-prerequisites' ? 'prerequisites' : 'details')
const legacyCombined = computed(() => route.name === undefined)
const backToStructure = computed(() => route.query.from === 'structure'
  ? { path: authoringDraftPath(draft.value.id, 'structure'), query: { lesson: lessonID(), search: typeof route.query.search === 'string' ? route.query.search : undefined } }
  : authoringDraftPath(draft.value.id, 'structure'))
const lessonTabs = computed(() => sections.map((section) => ({ label: section.label, to: sectionPath(section.id) })))

let requestVersion = 0
let active = true

const captureScope = useAuthoringAsyncScope(() => `${draft.value.id}/${lessonID()}`)

watch(() => route.params.lessonId, () => { void load() }, { immediate: true })
watch(currentSection, (section) => { void nextTick(() => document.getElementById(`authoring-lesson-${section}-title`)?.focus({ preventScroll: true })) })
onBeforeUnmount(() => { active = false })


function lessonID(): string { return typeof route.params.lessonId === 'string' ? route.params.lessonId : '' }
function openPreview() { void router.push({ name: 'authoring-draft-lesson-preview', params: { draftId: draft.value.id, lessonId: lessonID() } }) }
function sectionPath(section: Section) { const query = new URLSearchParams(Object.entries(route.query).filter((entry): entry is [string, string] => typeof entry[1] === 'string')); const suffix = query.toString(); return `/authoring/drafts/${encodeURIComponent(draft.value.id)}/lessons/${encodeURIComponent(lessonID())}/${section}${suffix ? `?${suffix}` : ''}` }
function toForm(lesson: AuthoringLessonDetail): Form {
  return {
    title: lesson.title,
    description: lesson.description,
    objectives: [...lesson.objectives],
    estimatedDurationMinutes: lesson.estimated_duration_minutes === null ? '' : String(lesson.estimated_duration_minutes),
  }
}
function normalized(value: Form) {
  return {
    title: value.title,
    description: value.description,
    objectives: value.objectives.map((objective) => objective.trim()),
    estimatedDurationMinutes: value.estimatedDurationMinutes.trim() === '' ? null : Number(value.estimatedDurationMinutes),
  }
}
function replaceForm(next: Form) { Object.assign(form, { ...next, objectives: [...next.objectives] }) }
function clearErrors() { for (const key of Object.keys(errors)) delete errors[key]; formError.value = undefined }
function discardChanges() {
  if (saving.value || reloading.value || !original.value) return
  replaceForm(original.value)
  clearErrors()
  saveMessage.value = undefined
  conflict.value = false
}

function validate(): boolean {
  clearErrors()
  if (!form.title.trim()) errors.title = t('authoring.details.validationTitle')
  if (!form.description.trim()) errors.description = t('authoring.details.validationDescription')
  if (!form.objectives.length || form.objectives.some((objective) => !objective.trim())) {
    errors.objectives = t('authoring.details.validationObjectives')
    form.objectives.forEach((objective, index) => { if (!objective.trim()) errors[`objective-${index}`] = t('authoring.details.validationObjective') })
  }
  const duration = form.estimatedDurationMinutes.trim()
  if (duration && (!/^[1-9][0-9]*$/.test(duration) || Number(duration) > 1440)) errors.estimatedDurationMinutes = t('authoring.details.validationDuration')
  return Object.keys(errors).length === 0
}

function patch(lesson: AuthoringLessonDetail): AuthoringLessonMetadataPatch {
  const before = normalized(original.value!)
  const after = normalized(form)
  const result: AuthoringLessonMetadataPatch = { expectedLessonRevision: lesson.revision }
  if (after.title !== before.title) result.title = after.title
  if (after.description !== before.description) result.description = after.description
  if (JSON.stringify(after.objectives) !== JSON.stringify(before.objectives)) result.objectives = after.objectives
  if (after.estimatedDurationMinutes !== before.estimatedDurationMinutes) result.estimatedDurationMinutes = after.estimatedDurationMinutes
  return result
}

async function load() {
  const isCurrent = captureScope()
  const generation = ++requestVersion
  state.value = { kind: 'loading' }
  saving.value = false
  reloading.value = false
  saveMessage.value = undefined
  conflict.value = false
  clearErrors()
  try {
    const lesson = await getAuthoringLesson(draft.value.id, lessonID())
    if (!isCurrent()) return
    if (!active || generation !== requestVersion) return
    const nextForm = toForm(lesson)
    state.value = { kind: 'ready', lesson }
    original.value = nextForm
    replaceForm(nextForm)
  } catch (error) {
    if (!isCurrent()) return
    if (!active || generation !== requestVersion) return
    if (error instanceof InvalidAuthoringDraftIDError || (error instanceof APIProblemError && error.status === 404)) {
      markDraftUnavailable()
      return
    }
    state.value = { kind: 'unavailable' }
  }
}

async function save() {
  const isCurrent = captureScope()
  if (saving.value || reloading.value || conflict.value || state.value.kind !== 'ready' || !dirty.value || !validate()) return
  saving.value = true
  saveMessage.value = undefined
  try {
    const updated = await updateAuthoringLesson(draft.value.id, state.value.lesson.id, patch(state.value.lesson))
    if (!isCurrent()) return
    if (state.value.kind !== 'ready' || updated.revision < state.value.lesson.revision) return
    const lesson = { ...state.value.lesson, ...updated }
    const nextForm = toForm(lesson)
    state.value = { kind: 'ready', lesson }
    original.value = nextForm
    replaceForm(nextForm)
    replaceDraft({ ...draft.value, revision: updated.draftRevision })
    saveMessage.value = t('authoring.details.saved')
  } catch (error) {
    if (!isCurrent()) return
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
    if (error instanceof APIProblemError && error.status === 409) { conflict.value = true; return }
    formError.value = error instanceof APIProblemError && error.status === 400
      ? t('authoring.details.saveInvalid')
      : t('authoring.details.saveUnavailable')
  } finally { if (isCurrent()) saving.value = false }
}

async function reloadLatest() {
  const isCurrent = captureScope()
  if (reloading.value || saving.value) return
  reloading.value = true
  clearErrors()
  saveMessage.value = undefined
  try {
    const latest = await getAuthoringLesson(draft.value.id, lessonID())
    if (!isCurrent()) return
    if (state.value.kind === 'ready' && latest.revision < state.value.lesson.revision) {
      conflict.value = true
      return
    }
    const nextForm = toForm(latest)
    state.value = { kind: 'ready', lesson: latest }
    original.value = nextForm
    replaceForm(nextForm)
    conflict.value = false
  } catch (error) {
    if (!isCurrent()) return
    if (error instanceof InvalidAuthoringDraftIDError || (error instanceof APIProblemError && error.status === 404)) markDraftUnavailable()
    else formError.value = t('authoring.details.reloadUnavailable')
  } finally { if (isCurrent()) reloading.value = false }
}

function replaceLesson(lesson: AuthoringLessonDetail) {
  if (state.value.kind !== 'ready' || lesson.id !== state.value.lesson.id || lesson.revision < state.value.lesson.revision) return
  const metadataWasDirty = dirty.value
  state.value = { kind: 'ready', lesson }
  const nextForm = toForm(lesson)
  original.value = nextForm
  if (!metadataWasDirty) replaceForm(nextForm)
}

function applyPrerequisites(result: { lesson: import('../api/generated').components['schemas']['AuthoringLessonMutationResponse']; prerequisiteKeys: string[] }) {
  if (state.value.kind !== 'ready') return
  if (result.lesson.id !== state.value.lesson.id || result.lesson.revision < state.value.lesson.revision) return
  const lesson = { ...state.value.lesson, ...result.lesson, recommended_prerequisite_keys: result.prerequisiteKeys }
  state.value = { kind: 'ready', lesson }
  replaceDraft({ ...draft.value, revision: result.lesson.draftRevision })
}

function applyContent(result: import('../authoring/authoring').AuthoringLessonContentMutation) {
  if (state.value.kind !== 'ready') return
  if (result.lesson.id !== state.value.lesson.id || result.lesson.revision < state.value.lesson.revision) return
  const lesson = { ...state.value.lesson, ...result.lesson, content: result.content }
  state.value = { kind: 'ready', lesson }
  replaceDraft({ ...draft.value, revision: result.lesson.draftRevision })
}
</script>
