<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { authoringDraftAssessmentPath, createAuthoringAssessment, listAuthoringDraftAssessments, type AuthoringAssessmentList } from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'

type State = { kind: 'loading' } | { kind: 'ready'; page: AuthoringAssessmentList } | { kind: 'unavailable' }
const { draft, markDraftUnavailable } = useAuthoringDraftContext()
const router = useRouter()
const state = ref<State>({ kind: 'loading' })
const creating = ref(false)
const title = ref('New assessment')
const createError = ref<string>()
let requestVersion = 0
let active = true
const captureScope = useAuthoringAsyncScope(() => draft.value.id)

watch(() => draft.value.id, () => { void load(0) }, { immediate: true })
onBeforeUnmount(() => { active = false })

async function load(offset = 0) {
  const isCurrent = captureScope(); const version = ++requestVersion
  if (state.value.kind !== 'ready') state.value = { kind: 'loading' }
  try {
    const page = await listAuthoringDraftAssessments(draft.value.id, 20, offset)
    if (!isCurrent() || !active || version !== requestVersion) return
    state.value = { kind: 'ready', page }
  } catch (error) {
    if (!isCurrent() || !active || version !== requestVersion) return
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
    state.value = { kind: 'unavailable' }
  }
}

async function create() {
  const isCurrent = captureScope()
  if (creating.value || !title.value.trim()) { createError.value = 'Enter an Assessment title.'; return }
  creating.value = true; createError.value = undefined
  try {
    const assessment = await createAuthoringAssessment(draft.value.id, { title: title.value.trim(), questions: [] })
    if (!isCurrent() || !active) return
    void router.push(authoringDraftAssessmentPath(draft.value.id, assessment.assessmentKey))
  } catch (error) {
    if (!isCurrent() || !active) return
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
    createError.value = error instanceof APIProblemError && error.status === 400
      ? 'We couldn’t create this Assessment. Check the title and try again.'
      : 'We couldn’t create this Assessment right now. Please try again.'
  } finally { if (isCurrent() && active) creating.value = false }
}
</script>

<template>
  <section class="authoring-section authoring-assessments" aria-labelledby="authoring-assessments-title">
    <header><h2 id="authoring-assessments-title">Assessments</h2><p class="authoring-section__intro">Create and edit the deterministic knowledge checks available in this Draft.</p></header>
    <form class="authoring-assessments__create" :aria-busy="creating" @submit.prevent="create">
      <h3>Create Assessment</h3>
      <BtgFormField label="Assessment title" required :error="createError" v-slot="{ controlId, describedBy, invalid }"><input :id="controlId" v-model="title" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" maxlength="240" required /></BtgFormField>
      <BtgButton type="submit" :disabled="creating">{{ creating ? 'Creating…' : 'Create Assessment' }}</BtgButton>
    </form>
    <p v-if="state.kind === 'loading'" role="status">Loading Assessments…</p>
    <div v-else-if="state.kind === 'unavailable'" class="authoring-assessments__state"><p>We couldn’t load Assessments right now.</p><BtgButton variant="secondary" @click="load(0)">Try again</BtgButton></div>
    <template v-else>
      <p v-if="!state.page.items.length" class="authoring-assessments__empty">No Assessments yet. Create the first one when you are ready.</p>
      <ol v-else class="authoring-assessments__list">
        <li v-for="assessment in state.page.items" :key="assessment.assessmentKey"><article><h3>{{ assessment.title }}</h3><dl><div><dt>Questions</dt><dd>{{ assessment.questionCount }}</dd></div><div><dt>Revision</dt><dd>{{ assessment.revision }}</dd></div><div><dt>Updated</dt><dd><time :datetime="assessment.updatedAt">{{ new Date(assessment.updatedAt).toLocaleString() }}</time></dd></div></dl><RouterLink :to="authoringDraftAssessmentPath(draft.id, assessment.assessmentKey)">Edit {{ assessment.title }}</RouterLink></article></li>
      </ol>
      <p v-if="state.page.total > state.page.limit" class="authoring-assessments__pagination"><BtgButton variant="secondary" :disabled="state.page.offset === 0" @click="load(Math.max(0, state.page.offset - state.page.limit))">Previous</BtgButton><BtgButton variant="secondary" :disabled="state.page.offset + state.page.limit >= state.page.total" @click="load(state.page.offset + state.page.limit)">Next</BtgButton></p>
    </template>
  </section>
</template>
