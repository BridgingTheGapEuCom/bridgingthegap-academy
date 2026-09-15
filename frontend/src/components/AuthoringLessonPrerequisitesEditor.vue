<template>
  <section class="authoring-prerequisites" aria-labelledby="authoring-prerequisites-title">
    <header>
      <h3 id="authoring-prerequisites-title">Recommended prerequisites</h3>
      <p>These are advisory recommendations. They do not restrict access to this Lesson.</p>
    </header>

    <div v-if="state === 'loading'" class="authoring-prerequisites__state" role="status">Loading available Lessons…</div>
    <div v-else-if="state === 'unavailable'" class="authoring-prerequisites__state">
      <p>We couldn’t load available Lessons right now.</p>
      <BtgButton variant="secondary" @click="loadCandidates">Try again</BtgButton>
    </div>

    <form v-else :aria-busy="saving" novalidate @submit.prevent="save">
      <p v-if="formError" class="authoring-prerequisites__error" role="alert">{{ formError }}</p>
      <p v-if="saveMessage" class="authoring-prerequisites__status" role="status">{{ saveMessage }}</p>
      <div v-if="conflict" class="authoring-prerequisites__conflict" role="status">
        <p>This Lesson changed elsewhere. Your recommended prerequisites are still here, but you need to reload the latest Lesson before saving again.</p>
        <BtgButton variant="secondary" :disabled="reloading" @click="reloadLatest">{{ reloading ? 'Reloading…' : 'Reload latest Lesson' }}</BtgButton>
      </div>

      <div v-if="availableCandidates.length" class="authoring-prerequisites__add">
        <BtgFormField label="Available Lessons" description="Choose a Lesson to recommend before this one." v-slot="{ controlId, describedBy }">
          <select :id="controlId" v-model="selectedKey" :aria-describedby="describedBy">
            <option value="">Choose a Lesson</option>
            <option v-for="candidate in availableCandidates" :key="candidate.stable_key" :value="candidate.stable_key">{{ candidate.title }} ({{ candidate.stable_key }})</option>
          </select>
        </BtgFormField>
        <BtgButton variant="secondary" :disabled="!selectedKey" @click="addPrerequisite">Add recommended prerequisite</BtgButton>
      </div>
      <p v-else class="authoring-prerequisites__empty">No other Lessons are available to recommend.</p>

      <ol v-if="selectedKeys.length" class="authoring-prerequisites__list" aria-label="Ordered recommended prerequisites">
        <li v-for="(key, index) in selectedKeys" :key="key">
          <div><strong>{{ candidateFor(key)?.title || key }}</strong> <code>{{ key }}</code></div>
          <div class="authoring-prerequisites__actions">
            <BtgButton variant="secondary" :disabled="index === 0" :aria-label="`Move ${candidateFor(key)?.title || key} up`" @click="move(index, -1)">Move up</BtgButton>
            <BtgButton variant="secondary" :disabled="index === selectedKeys.length - 1" :aria-label="`Move ${candidateFor(key)?.title || key} down`" @click="move(index, 1)">Move down</BtgButton>
            <BtgButton variant="secondary" :aria-label="`Remove ${candidateFor(key)?.title || key}`" @click="removePrerequisite(index)">Remove</BtgButton>
          </div>
        </li>
      </ol>
      <p v-else class="authoring-prerequisites__empty">No recommended prerequisites yet.</p>

      <p v-if="dirty" class="authoring-prerequisites__unsaved" role="status">You have unsaved prerequisite changes.</p>
      <div class="authoring-prerequisites__save"><BtgButton type="submit" :disabled="!dirty || saving || conflict">{{ saving ? 'Saving…' : 'Save recommended prerequisites' }}</BtgButton></div>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { APIProblemError } from '../api/client'
import { getAuthoringLesson, getAuthoringStructure, InvalidAuthoringDraftIDError, replaceAuthoringLessonPrerequisites, type AuthoringLessonDetail, type AuthoringLessonSummary } from '../authoring/authoring'
import BtgButton from './BtgButton.vue'
import BtgFormField from './BtgFormField.vue'

const props = defineProps<{ draftId: string; lesson: AuthoringLessonDetail }>()
const emit = defineEmits<{
  saved: [result: { lesson: import('../api/generated').components['schemas']['AuthoringLessonMutationResponse']; prerequisiteKeys: string[] }]
  replaceLesson: [lesson: AuthoringLessonDetail]
  unavailable: []
}>()

const state = ref<'loading' | 'ready' | 'unavailable'>('loading')
const candidates = ref<AuthoringLessonSummary[]>([])
const originalKeys = ref<string[]>([])
const selectedKeys = ref<string[]>([])
const selectedKey = ref('')
const saving = ref(false)
const reloading = ref(false)
const conflict = ref(false)
const formError = ref<string>()
const saveMessage = ref<string>()
const dirty = computed(() => JSON.stringify(selectedKeys.value) !== JSON.stringify(originalKeys.value))
const candidateByKey = computed(() => new Map(candidates.value.map((candidate) => [candidate.stable_key, candidate])))
const availableCandidates = computed(() => candidates.value.filter((candidate) => candidate.stable_key !== props.lesson.stable_key && !selectedKeys.value.includes(candidate.stable_key)))

let requestVersion = 0
let active = true

watch(() => props.lesson.id, () => {
  originalKeys.value = [...props.lesson.recommended_prerequisite_keys]
  selectedKeys.value = [...props.lesson.recommended_prerequisite_keys]
  selectedKey.value = ''
  conflict.value = false
  void loadCandidates()
}, { immediate: true })
onBeforeUnmount(() => { active = false })

// A reload in another Lesson section may bring a newer prerequisite snapshot.
// Keep independent local edits, but require an explicit reload if that list changed.
watch(() => JSON.stringify(props.lesson.recommended_prerequisite_keys), () => {
  if (JSON.stringify(props.lesson.recommended_prerequisite_keys) === JSON.stringify(originalKeys.value)) return
  if (dirty.value) conflict.value = true
  else {
    originalKeys.value = [...props.lesson.recommended_prerequisite_keys]
    selectedKeys.value = [...props.lesson.recommended_prerequisite_keys]
  }
})

function candidateFor(key: string) { return candidateByKey.value.get(key) }
function addPrerequisite() {
  if (!selectedKey.value || selectedKey.value === props.lesson.stable_key || selectedKeys.value.includes(selectedKey.value) || !candidateFor(selectedKey.value)) return
  selectedKeys.value.push(selectedKey.value)
  selectedKey.value = ''
}
function removePrerequisite(index: number) { selectedKeys.value.splice(index, 1) }
function move(index: number, direction: -1 | 1) {
  const destination = index + direction
  if (destination < 0 || destination >= selectedKeys.value.length) return
  const next = [...selectedKeys.value]
  const [key] = next.splice(index, 1)
  next.splice(destination, 0, key!)
  selectedKeys.value = next
}
function clearError() { formError.value = undefined }

async function loadCandidates() {
  const generation = ++requestVersion
  state.value = 'loading'
  try {
    const structure = await getAuthoringStructure(props.draftId)
    if (!active || generation !== requestVersion) return
    candidates.value = structure.modules.flatMap((module) => module.lessons)
    state.value = 'ready'
  } catch (error) {
    if (!active || generation !== requestVersion) return
    if (error instanceof InvalidAuthoringDraftIDError || (error instanceof APIProblemError && error.status === 404)) { emit('unavailable'); return }
    state.value = 'unavailable'
  }
}

async function save() {
  if (saving.value || conflict.value || !dirty.value) return
  if (selectedKeys.value.includes(props.lesson.stable_key) || new Set(selectedKeys.value).size !== selectedKeys.value.length || selectedKeys.value.some((key) => !candidateFor(key))) {
    formError.value = 'Reload the available Lessons before saving these recommendations.'
    return
  }
  saving.value = true
  clearError()
  saveMessage.value = undefined
  try {
    const keys = [...selectedKeys.value]
    const lesson = await replaceAuthoringLessonPrerequisites(props.draftId, props.lesson.id, { expectedLessonRevision: props.lesson.revision, prerequisiteLessonKeys: keys })
    originalKeys.value = keys
    emit('saved', { lesson, prerequisiteKeys: keys })
    saveMessage.value = 'Recommended prerequisites saved.'
  } catch (error) {
    if (error instanceof APIProblemError && error.status === 404) { emit('unavailable'); return }
    if (error instanceof APIProblemError && error.status === 409) { conflict.value = true; return }
    formError.value = error instanceof APIProblemError && error.status === 400
      ? 'We couldn’t save these recommendations. Check the selected Lessons and try again.'
      : 'We couldn’t save recommended prerequisites right now. Please try again.'
  } finally { saving.value = false }
}

async function reloadLatest() {
  if (reloading.value) return
  reloading.value = true
  clearError()
  saveMessage.value = undefined
  try {
    const latest = await getAuthoringLesson(props.draftId, props.lesson.id)
    originalKeys.value = [...latest.recommended_prerequisite_keys]
    selectedKeys.value = [...latest.recommended_prerequisite_keys]
    conflict.value = false
    emit('replaceLesson', latest)
  } catch (error) {
    if (error instanceof InvalidAuthoringDraftIDError || (error instanceof APIProblemError && error.status === 404)) emit('unavailable')
    else formError.value = 'We couldn’t reload this Lesson right now. Please try again.'
  } finally { reloading.value = false }
}
</script>
