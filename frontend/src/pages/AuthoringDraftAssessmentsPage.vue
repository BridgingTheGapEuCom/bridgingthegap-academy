<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { authoringDraftAssessmentPath, createAuthoringAssessment, listAuthoringDraftAssessments, type AuthoringAssessmentList } from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import { useApplicationFormatters } from '../i18n/formatters'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgPanel from '../components/BtgPanel.vue'
import BtgTextInput from '../components/BtgTextInput.vue'
import AuthoringPageTitle from '../components/AuthoringPageTitle.vue'
import AuthoringSection from '../components/AuthoringSection.vue'

type State = { kind: 'loading' } | { kind: 'ready'; page: AuthoringAssessmentList } | { kind: 'unavailable' }
const { draft, markDraftUnavailable } = useAuthoringDraftContext()
const { t } = useI18n()
const { date } = useApplicationFormatters()
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
  if (creating.value || !title.value.trim()) { createError.value = t('authoring.assessments.createInvalid'); return }
  creating.value = true; createError.value = undefined
  try {
    const assessment = await createAuthoringAssessment(draft.value.id, { title: title.value.trim(), questions: [] })
    if (!isCurrent() || !active) return
    void router.push(authoringDraftAssessmentPath(draft.value.id, assessment.assessmentKey))
  } catch (error) {
    if (!isCurrent() || !active) return
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
    createError.value = error instanceof APIProblemError && error.status === 400
      ? t('authoring.assessments.createInvalidRequest')
      : t('authoring.assessments.createUnavailable')
  } finally { if (isCurrent() && active) creating.value = false }
}
</script>

<template>
  <section class="authoring-section authoring-assessments" aria-labelledby="authoring-assessments-title">
    <AuthoringPageTitle id="authoring-assessments-title" :title="t('authoring.assessments.title')" :description="t('authoring.assessments.description')" />
    <form class="authoring-assessments__create" :aria-busy="creating" novalidate @submit.prevent="create">
      <AuthoringSection heading-id="authoring-create-assessment-title" :title="t('authoring.assessments.create')">
        <BtgFormField :label="t('authoring.assessments.assessmentTitle')" required :error="createError" v-slot="{ controlId, describedBy, invalid }">
          <BtgTextInput :id="controlId" v-model="title" :aria-describedby="describedBy" :invalid="invalid" maxlength="240" required />
        </BtgFormField>
        <BtgButton type="submit" :disabled="creating" leading-icon="add">{{ creating ? t('authoring.assessments.creating') : t('authoring.assessments.create') }}</BtgButton>
      </AuthoringSection>
    </form>
    <BtgPanel v-if="state.kind === 'loading'" variant="subtle" padding="compact"><p role="status">{{ t('authoring.assessments.loading') }}</p></BtgPanel>
    <BtgPanel v-else-if="state.kind === 'unavailable'" variant="subtle" padding="compact" class="authoring-assessments__state"><p>{{ t('authoring.assessments.unavailable') }}</p><BtgButton variant="secondary" @click="load(0)">{{ t('authoring.assessments.tryAgain') }}</BtgButton></BtgPanel>
    <template v-else>
      <BtgPanel v-if="!state.page.items.length" variant="subtle" padding="compact"><p class="authoring-assessments__empty">{{ t('authoring.assessments.empty') }}</p></BtgPanel>
      <ol v-else class="authoring-assessments__list">
        <li v-for="assessment in state.page.items" :key="assessment.assessmentKey">
          <BtgPanel as="article" padding="compact"><h3>{{ assessment.title }}</h3><dl><div><dt>{{ t('authoring.assessments.questions', assessment.questionCount) }}</dt><dd>{{ assessment.questionCount }}</dd></div><div><dt>{{ t('authoring.assessments.revision') }}</dt><dd>{{ assessment.revision }}</dd></div><div><dt>{{ t('authoring.assessments.updated') }}</dt><dd><time :datetime="assessment.updatedAt">{{ date(assessment.updatedAt, 'dateTime') }}</time></dd></div></dl><RouterLink :to="authoringDraftAssessmentPath(draft.id, assessment.assessmentKey)">{{ t('authoring.assessments.edit', { title: assessment.title }) }}</RouterLink></BtgPanel>
        </li>
      </ol>
      <nav v-if="state.page.total > state.page.limit" class="authoring-assessments__pagination" :aria-label="t('authoring.assessments.title')"><BtgButton variant="secondary" :disabled="state.page.offset === 0" @click="load(Math.max(0, state.page.offset - state.page.limit))">{{ t('authoring.assessments.previous') }}</BtgButton><BtgButton variant="secondary" :disabled="state.page.offset + state.page.limit >= state.page.total" @click="load(state.page.offset + state.page.limit)">{{ t('authoring.assessments.next') }}</BtgButton></nav>
    </template>
  </section>
</template>
