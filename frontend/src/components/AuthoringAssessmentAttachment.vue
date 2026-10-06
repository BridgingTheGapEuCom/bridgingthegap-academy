<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { APIProblemError } from '../api/client'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { listAuthoringDraftAssessments, type AuthoringAssessmentSummary } from '../authoring/authoring'
import BtgButton from './BtgButton.vue'
import BtgBadge from './BtgBadge.vue'
import BtgPanel from './BtgPanel.vue'

export interface AuthoringAssessmentAttachmentCopy {
  title: string
  selected: string
  selectedUnavailable: string
  loading: string
  loadUnavailable: string
  retry: string
  empty: string
  available: string
  use: (title: string) => string
  questions: (count: number) => string
  updated: (date: string) => string
  previous: string
  next: string
  pagination: (from: number, to: number, total: number) => string
}

const props = defineProps<{ draftId: string; lessonId: string; blockKey: string; currentAssessmentKey: string; copy: AuthoringAssessmentAttachmentCopy }>()
const emit = defineEmits<{ attached: [assessmentKey: string]; unavailable: [] }>()
const page = ref<AuthoringAssessmentSummary[]>([]); const limit = ref(20); const offset = ref(0); const total = ref(0)
const loading = ref(false); const error = ref<string>(); let active = true; let version = 0
const currentKnown = computed(() => page.value.some((assessment) => assessment.assessmentKey === props.currentAssessmentKey))
const captureScope = useAuthoringAsyncScope(() => `${props.draftId}/${props.lessonId}/${props.blockKey}`)
watch(() => [props.draftId, props.lessonId, props.blockKey], () => { page.value = []; offset.value = 0; total.value = 0; error.value = undefined; void load(0) }, { immediate: true })
onBeforeUnmount(() => { active = false })
async function load(nextOffset = offset.value) {
  if (loading.value) return
  const isCurrent = captureScope(); const generation = ++version; loading.value = true; error.value = undefined
  try { const response = await listAuthoringDraftAssessments(props.draftId, limit.value, nextOffset); if (!isCurrent() || !active || generation !== version) return; page.value = response.items; limit.value = response.limit; offset.value = response.offset; total.value = response.total }
  catch (reason) { if (!isCurrent() || !active || generation !== version) return; if (reason instanceof APIProblemError && reason.status === 404) { emit('unavailable'); return }; error.value = props.copy.loadUnavailable }
  finally { if (isCurrent() && active && generation === version) loading.value = false }
}
function select(assessment: AuthoringAssessmentSummary) { emit('attached', assessment.assessmentKey) }
</script>

<template>
  <section class="authoring-assessment-attachment" aria-labelledby="authoring-assessment-attachment-title">
    <h4 id="authoring-assessment-attachment-title">{{ copy.title }}</h4>
    <BtgBadge v-if="currentAssessmentKey && currentKnown">{{ copy.selected }}</BtgBadge>
    <p v-else-if="currentAssessmentKey">{{ copy.selectedUnavailable }}</p>
    <p v-if="loading" role="status">{{ copy.loading }}</p>
    <p v-else-if="error" role="alert">{{ error }} <BtgButton variant="secondary" @click="load(0)">{{ copy.retry }}</BtgButton></p>
    <p v-else-if="!page.length">{{ copy.empty }}</p>
    <ul v-else :aria-label="copy.available"><li v-for="assessment in page" :key="assessment.assessmentKey"><BtgPanel padding="compact"><BtgButton variant="secondary" :aria-pressed="assessment.assessmentKey === currentAssessmentKey" @click="select(assessment)">{{ copy.use(assessment.title) }}</BtgButton><span>{{ copy.questions(assessment.questionCount) }} · {{ copy.updated(assessment.updatedAt) }}</span></BtgPanel></li></ul>
    <nav v-if="total > limit" :aria-label="copy.pagination(offset + 1, Math.min(offset + limit, total), total)"><BtgButton variant="secondary" :disabled="loading || offset === 0" @click="load(Math.max(0, offset - limit))">{{ copy.previous }}</BtgButton><BtgButton variant="secondary" :disabled="loading || offset + limit >= total" @click="load(offset + limit)">{{ copy.next }}</BtgButton></nav>
  </section>
</template>
