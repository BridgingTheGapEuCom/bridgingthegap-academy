<template>
  <section class="authoring-section authoring-structure" aria-labelledby="authoring-structure-title">
    <header class="authoring-structure__header">
      <h2 id="authoring-structure-title">Structure</h2>
      <p class="authoring-section__intro">Arrange Modules and Lessons in the order learners will encounter them.</p>
    </header>

    <p v-if="state.kind === 'loading'" role="status">Loading Draft structure…</p>
    <div v-else-if="state.kind === 'unavailable'" class="authoring-structure__state">
      <p>We couldn’t load this Draft structure right now.</p>
      <BtgButton variant="secondary" @click="loadStructure">Try again</BtgButton>
    </div>

    <template v-else-if="state.kind === 'ready'">
      <p v-if="message" class="authoring-structure__status" role="status">{{ message }}</p>
      <p v-if="error" class="authoring-structure__error" role="alert">{{ error }}</p>
      <div v-if="conflict" class="authoring-structure__conflict" role="status">
        <p>This Draft changed elsewhere. Your structure has not been overwritten.</p>
        <BtgButton variant="secondary" :disabled="busy" @click="reloadLatest">Reload latest Draft</BtgButton>
      </div>

      <form class="authoring-structure__create-module" @submit.prevent="createModule">
        <h3>Add module</h3>
        <BtgFormField label="Module stable key" required description="A stable identifier that cannot later be changed." :error="moduleCreateErrors.stableKey" v-slot="{ controlId, describedBy, invalid }">
          <BtgTextInput :id="controlId" v-model="moduleCreate.stableKey" :aria-describedby="describedBy" :invalid="invalid" required />
        </BtgFormField>
        <BtgFormField label="Module title" required :error="moduleCreateErrors.title" v-slot="{ controlId, describedBy, invalid }">
          <BtgTextInput :id="controlId" v-model="moduleCreate.title" :aria-describedby="describedBy" :invalid="invalid" required />
        </BtgFormField>
        <BtgFormField label="Module description" v-slot="{ controlId, describedBy }">
          <textarea :id="controlId" v-model="moduleCreate.description" :aria-describedby="describedBy" />
        </BtgFormField>
        <BtgButton type="submit" :disabled="busy || conflict">Create module</BtgButton>
      </form>

      <p v-if="!state.structure.modules.length" class="authoring-structure__empty">No Modules yet. Add the first Module to begin arranging this Draft.</p>
      <div v-else class="authoring-structure__modules">
        <AuthoringStructureModule
          v-for="(module, moduleIndex) in state.structure.modules"
          :key="module.id"
          :draft-id="draft.id"
          :module="module"
          :module-index="moduleIndex"
          :module-count="state.structure.modules.length"
          :previous-module="state.structure.modules[moduleIndex - 1]"
          :next-module="state.structure.modules[moduleIndex + 1]"
          :busy="busy || conflict"
          @update-module="updateModule"
          @move-module="moveModule(moduleIndex, $event)"
          @delete-module="deleteModule"
          @create-lesson="createLesson"
          @move-lesson="moveLesson"
          @delete-lesson="deleteLesson"
        />
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, reactive, ref, watch } from 'vue'
import { APIProblemError } from '../api/client'
import {
  createAuthoringLesson,
  createAuthoringModule,
  deleteAuthoringLesson,
  deleteAuthoringModule,
  getAuthoringDraft,
  getAuthoringStructure,
  reorderAuthoringLessons,
  reorderAuthoringModules,
  updateAuthoringModule,
  type AuthoringLessonCreate,
  type AuthoringLessonSummary,
  type AuthoringModuleSummary,
  type AuthoringStructure,
} from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import AuthoringStructureModule from '../components/AuthoringStructureModule.vue'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgTextInput from '../components/BtgTextInput.vue'

type State = { kind: 'loading' } | { kind: 'ready'; structure: AuthoringStructure } | { kind: 'unavailable' }

const { draft, replaceDraft, markDraftUnavailable } = useAuthoringDraftContext()
const state = ref<State>({ kind: 'loading' })
const busy = ref(false)
const conflict = ref(false)
const error = ref<string>()
const message = ref<string>()
const moduleCreate = reactive({ stableKey: '', title: '', description: '' })
const moduleCreateErrors = reactive<Record<string, string | undefined>>({})
let requestVersion = 0
let active = true

watch(() => draft.value.id, () => { void loadStructure() }, { immediate: true })
onBeforeUnmount(() => { active = false })

async function loadStructure() {
  const version = ++requestVersion
  state.value = { kind: 'loading' }
  try {
    const structure = await getAuthoringStructure(draft.value.id)
    if (!active || version !== requestVersion) return
    state.value = { kind: 'ready', structure }
  } catch (cause) {
    if (!active || version !== requestVersion) return
    if (cause instanceof APIProblemError && cause.status === 404) { markDraftUnavailable(); return }
    state.value = { kind: 'unavailable' }
  }
}

async function reloadLatest() {
  busy.value = true
  error.value = undefined
  try {
    const [latestDraft, structure] = await Promise.all([getAuthoringDraft(draft.value.id), getAuthoringStructure(draft.value.id)])
    replaceDraft(latestDraft)
    state.value = { kind: 'ready', structure }
    conflict.value = false
    message.value = 'The latest Draft structure has been loaded.'
  } catch (cause) {
    handleError(cause, 'We couldn’t reload this Draft right now. Please try again.')
  } finally {
    busy.value = false
  }
}

function validateStableKey(value: string): boolean {
  return value.length >= 3 && value.length <= 96 && /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(value)
}

async function createModule() {
  for (const key of Object.keys(moduleCreateErrors)) delete moduleCreateErrors[key]
  if (!validateStableKey(moduleCreate.stableKey)) moduleCreateErrors.stableKey = 'Use lowercase letters, numbers, and hyphens.'
  if (!moduleCreate.title.trim()) moduleCreateErrors.title = 'Enter a module title.'
  if (Object.keys(moduleCreateErrors).length || state.value.kind !== 'ready') return
  await mutate(async () => {
    const result = await createAuthoringModule(draft.value.id, {
      expectedDraftRevision: draft.value.revision,
      stableKey: moduleCreate.stableKey,
      title: moduleCreate.title,
      ...(moduleCreate.description ? { description: moduleCreate.description } : {}),
      position: state.value.kind === 'ready' ? state.value.structure.modules.length : 0,
    })
    moduleCreate.stableKey = ''
    moduleCreate.title = ''
    moduleCreate.description = ''
    await commit(result.draftRevision, 'Module created.')
  })
}

async function updateModule(module: AuthoringModuleSummary, patch: { title?: string; description?: string }) {
  await mutate(async () => {
    const result = await updateAuthoringModule(draft.value.id, module.id, { expectedModuleRevision: module.revision, ...patch })
    await commit(result.draftRevision, 'Module updated.')
  })
}

async function moveModule(index: number, delta: -1 | 1) {
  if (state.value.kind !== 'ready') return
  const moduleIDs = state.value.structure.modules.map((module) => module.id)
  const target = index + delta
  if (target < 0 || target >= moduleIDs.length) return
  ;[moduleIDs[index], moduleIDs[target]] = [moduleIDs[target], moduleIDs[index]]
  await mutate(async () => {
    const result = await reorderAuthoringModules(draft.value.id, draft.value.revision, moduleIDs)
    await commit(result.draftRevision, 'Module order updated.')
  })
}

async function deleteModule(module: AuthoringModuleSummary) {
  if (module.lessons.length) { error.value = 'Move or delete this Module’s Lessons before deleting the Module.'; return }
  await mutate(async () => {
    const result = await deleteAuthoringModule(draft.value.id, module.id, draft.value.revision, module.revision)
    await commit(result.draftRevision, 'Empty Module deleted.')
  })
}

async function createLesson(moduleID: string, input: AuthoringLessonCreate) {
  await mutate(async () => {
    const result = await createAuthoringLesson(draft.value.id, moduleID, { ...input, expectedDraftRevision: draft.value.revision })
    await commit(result.draftRevision, 'Lesson created.')
  })
}

async function moveLesson(input: { lessonID: string; moduleID: string; position: number }) {
  if (state.value.kind !== 'ready') return
  const modules = state.value.structure.modules.map((module) => ({ moduleId: module.id, lessonIds: [...module.lessons.map((lesson) => lesson.id)] }))
  const source = modules.find((module) => module.lessonIds.includes(input.lessonID))
  const target = modules.find((module) => module.moduleId === input.moduleID)
  if (!source || !target) return
  source.lessonIds.splice(source.lessonIds.indexOf(input.lessonID), 1)
  const position = Math.max(0, Math.min(input.position, target.lessonIds.length))
  target.lessonIds.splice(position, 0, input.lessonID)
  await mutate(async () => {
    const result = await reorderAuthoringLessons(draft.value.id, draft.value.revision, modules)
    await commit(result.draftRevision, 'Lesson order updated.')
  })
}

async function deleteLesson(lesson: AuthoringLessonSummary) {
  await mutate(async () => {
    const result = await deleteAuthoringLesson(draft.value.id, lesson.id, draft.value.revision, lesson.revision)
    await commit(result.draftRevision, 'Lesson deleted.')
  })
}

async function mutate(operation: () => Promise<void>) {
  if (busy.value || conflict.value) return
  busy.value = true
  error.value = undefined
  message.value = undefined
  try {
    await operation()
  } catch (cause) {
    handleError(cause, 'We couldn’t save this structure right now. Please try again.')
  } finally {
    busy.value = false
  }
}

async function commit(revision: number, successMessage: string) {
  replaceDraft({ ...draft.value, revision })
  await loadStructure()
  message.value = successMessage
}

function handleError(cause: unknown, fallback: string) {
  if (cause instanceof APIProblemError && cause.status === 404) { markDraftUnavailable(); return }
  if (cause instanceof APIProblemError && cause.status === 409) { conflict.value = true; return }
  error.value = cause instanceof APIProblemError && cause.status === 400
    ? 'We couldn’t save these changes. Check the fields and try again.'
    : fallback
}
</script>
