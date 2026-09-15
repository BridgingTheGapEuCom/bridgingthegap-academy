<template>
  <section class="authoring-section authoring-lesson-editor" aria-labelledby="authoring-lesson-title">
    <RouterLink :to="authoringDraftPath(draft.id, 'structure')">Back to structure</RouterLink>

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
      <header class="authoring-lesson-editor__header">
        <p class="authoring-section__intro">Editing Lesson metadata in this Draft.</p>
        <h2 id="authoring-lesson-title">Lesson metadata</h2>
        <p class="authoring-lesson-editor__key"><span>Stable key</span> <code>{{ state.lesson.stable_key }}</code></p>
      </header>

      <form class="authoring-lesson-editor__form" :aria-busy="saving" novalidate @submit.prevent="save">
        <p v-if="formError" class="authoring-lesson-editor__error" role="alert">{{ formError }}</p>
        <p v-if="saveMessage" class="authoring-lesson-editor__status" role="status">{{ saveMessage }}</p>
        <div v-if="conflict" class="authoring-lesson-editor__conflict" role="status">
          <p>This Lesson changed elsewhere. Your edits are still here, but you need to reload the latest Lesson before saving again.</p>
          <BtgButton variant="secondary" :disabled="reloading" @click="reloadLatest">{{ reloading ? 'Reloading…' : 'Reload latest Lesson' }}</BtgButton>
        </div>

        <BtgFormField label="Title" required :error="errors.title" v-slot="{ controlId, describedBy, invalid }">
          <BtgTextInput :id="controlId" v-model="form.title" :aria-describedby="describedBy" :invalid="invalid" required />
        </BtgFormField>
        <BtgFormField label="Description" required :error="errors.description" v-slot="{ controlId, describedBy, invalid }">
          <textarea :id="controlId" v-model="form.description" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" required />
        </BtgFormField>

        <fieldset class="authoring-lesson-editor__objectives" :aria-describedby="errors.objectives ? 'authoring-lesson-objectives-error' : undefined">
          <legend>Learning objectives <span aria-hidden="true">*</span><span class="sr-only"> required</span></legend>
          <p>Describe what a learner should be able to do. The order is kept when you save.</p>
          <ol>
            <li v-for="(_objective, index) in form.objectives" :key="`objective-${index}`">
              <BtgFormField :label="`Learning objective ${index + 1}`" :error="errors[`objective-${index}`]">
                <template #default="{ controlId, describedBy, invalid }">
                  <BtgTextInput :id="controlId" v-model="form.objectives[index]" :aria-describedby="describedBy" :invalid="invalid" required />
                </template>
              </BtgFormField>
              <BtgButton variant="secondary" :disabled="form.objectives.length === 1" :aria-label="`Remove learning objective ${index + 1}`" @click="removeObjective(index)">Remove</BtgButton>
            </li>
          </ol>
          <p v-if="errors.objectives" id="authoring-lesson-objectives-error" class="btg-form-field__error">{{ errors.objectives }}</p>
          <BtgButton variant="secondary" @click="addObjective">Add learning objective</BtgButton>
        </fieldset>

        <BtgFormField label="Estimated duration (minutes)" description="Optional. Enter whole minutes." :error="errors.estimatedDurationMinutes" v-slot="{ controlId, describedBy, invalid }">
          <BtgTextInput :id="controlId" v-model="form.estimatedDurationMinutes" :aria-describedby="describedBy" :invalid="invalid" type="number" inputmode="numeric" min="1" max="1440" step="1" />
        </BtgFormField>

        <p v-if="dirty" class="authoring-lesson-editor__unsaved" role="status">You have unsaved changes.</p>
        <div class="authoring-lesson-editor__actions"><BtgButton type="submit" :disabled="!dirty || saving || conflict">{{ saving ? 'Saving…' : 'Save changes' }}</BtgButton></div>
      </form>

      <AuthoringLessonPrerequisitesEditor
        :draft-id="draft.id"
        :lesson="state.lesson"
        @saved="applyPrerequisites"
        @replace-lesson="replaceLesson"
        @unavailable="markDraftUnavailable"
      />
      <AuthoringLessonContentEditor :draft-id="draft.id" :lesson="state.lesson" @saved="applyContent" @replace-lesson="replaceLesson" @unavailable="markDraftUnavailable" />
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { APIProblemError } from '../api/client'
import { authoringDraftPath, getAuthoringLesson, InvalidAuthoringDraftIDError, updateAuthoringLesson, type AuthoringLessonDetail, type AuthoringLessonMetadataPatch } from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgTextInput from '../components/BtgTextInput.vue'
import AuthoringLessonPrerequisitesEditor from '../components/AuthoringLessonPrerequisitesEditor.vue'
import AuthoringLessonContentEditor from '../components/AuthoringLessonContentEditor.vue'

type Form = { title: string; description: string; objectives: string[]; estimatedDurationMinutes: string }
type State = { kind: 'loading' } | { kind: 'ready'; lesson: AuthoringLessonDetail } | { kind: 'unavailable' }

const route = useRoute()
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

let requestVersion = 0
let active = true

watch(() => route.params.lessonId, () => { void load() }, { immediate: true })
onBeforeUnmount(() => { active = false })

function lessonID(): string { return typeof route.params.lessonId === 'string' ? route.params.lessonId : '' }
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
function addObjective() { form.objectives.push('') }
function removeObjective(index: number) { if (form.objectives.length > 1) form.objectives.splice(index, 1) }

function validate(): boolean {
  clearErrors()
  if (!form.title.trim()) errors.title = 'Enter a title.'
  if (!form.description.trim()) errors.description = 'Enter a description.'
  if (!form.objectives.length || form.objectives.some((objective) => !objective.trim())) {
    errors.objectives = 'Enter a learning objective in every field.'
    form.objectives.forEach((objective, index) => { if (!objective.trim()) errors[`objective-${index}`] = 'Enter a learning objective.' })
  }
  const duration = form.estimatedDurationMinutes.trim()
  if (duration && (!/^[1-9][0-9]*$/.test(duration) || Number(duration) > 1440)) errors.estimatedDurationMinutes = 'Enter a whole number from 1 to 1440, or leave this blank.'
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
  const generation = ++requestVersion
  state.value = { kind: 'loading' }
  conflict.value = false
  clearErrors()
  try {
    const lesson = await getAuthoringLesson(draft.value.id, lessonID())
    if (!active || generation !== requestVersion) return
    const nextForm = toForm(lesson)
    state.value = { kind: 'ready', lesson }
    original.value = nextForm
    replaceForm(nextForm)
  } catch (error) {
    if (!active || generation !== requestVersion) return
    if (error instanceof InvalidAuthoringDraftIDError || (error instanceof APIProblemError && error.status === 404)) {
      markDraftUnavailable()
      return
    }
    state.value = { kind: 'unavailable' }
  }
}

async function save() {
  if (saving.value || conflict.value || state.value.kind !== 'ready' || !dirty.value || !validate()) return
  saving.value = true
  saveMessage.value = undefined
  try {
    const updated = await updateAuthoringLesson(draft.value.id, state.value.lesson.id, patch(state.value.lesson))
    const lesson = { ...state.value.lesson, ...updated }
    const nextForm = toForm(lesson)
    state.value = { kind: 'ready', lesson }
    original.value = nextForm
    replaceForm(nextForm)
    replaceDraft({ ...draft.value, revision: updated.draftRevision })
    saveMessage.value = 'Lesson metadata saved.'
  } catch (error) {
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
    if (error instanceof APIProblemError && error.status === 409) { conflict.value = true; return }
    formError.value = error instanceof APIProblemError && error.status === 400
      ? 'We couldn’t save these changes. Check the fields and try again.'
      : 'We couldn’t save this Lesson right now. Please try again.'
  } finally { saving.value = false }
}

async function reloadLatest() {
  if (reloading.value) return
  reloading.value = true
  clearErrors()
  saveMessage.value = undefined
  try {
    const latest = await getAuthoringLesson(draft.value.id, lessonID())
    const nextForm = toForm(latest)
    state.value = { kind: 'ready', lesson: latest }
    original.value = nextForm
    replaceForm(nextForm)
    conflict.value = false
  } catch (error) {
    if (error instanceof InvalidAuthoringDraftIDError || (error instanceof APIProblemError && error.status === 404)) markDraftUnavailable()
    else formError.value = 'We couldn’t reload this Lesson right now. Please try again.'
  } finally { reloading.value = false }
}

function replaceLesson(lesson: AuthoringLessonDetail) {
  const metadataWasDirty = dirty.value
  state.value = { kind: 'ready', lesson }
  const nextForm = toForm(lesson)
  original.value = nextForm
  if (!metadataWasDirty) replaceForm(nextForm)
}

function applyPrerequisites(result: { lesson: import('../api/generated').components['schemas']['AuthoringLessonMutationResponse']; prerequisiteKeys: string[] }) {
  if (state.value.kind !== 'ready') return
  const lesson = { ...state.value.lesson, ...result.lesson, recommended_prerequisite_keys: result.prerequisiteKeys }
  state.value = { kind: 'ready', lesson }
  replaceDraft({ ...draft.value, revision: result.lesson.draftRevision })
}

function applyContent(result: import('../authoring/authoring').AuthoringLessonContentMutation) {
  if (state.value.kind !== 'ready') return
  const lesson = { ...state.value.lesson, ...result.lesson, content: result.content }
  state.value = { kind: 'ready', lesson }
  replaceDraft({ ...draft.value, revision: result.lesson.draftRevision })
}
</script>
