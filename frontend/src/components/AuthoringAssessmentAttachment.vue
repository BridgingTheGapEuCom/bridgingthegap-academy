<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { APIProblemError } from '../api/client'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { listAuthoringDraftAssessments, type AuthoringAssessmentSummary } from '../authoring/authoring'
import BtgButton from './BtgButton.vue'

const props = defineProps<{ draftId: string; lessonId: string; blockKey: string; currentAssessmentKey: string }>()
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
  catch (reason) { if (!isCurrent() || !active || generation !== version) return; if (reason instanceof APIProblemError && reason.status === 404) { emit('unavailable'); return }; error.value = 'We couldn’t load Assessments right now. Try again.' }
  finally { if (isCurrent() && active && generation === version) loading.value = false }
}
function select(assessment: AuthoringAssessmentSummary) { emit('attached', assessment.assessmentKey) }
</script>

<template>
  <section class="authoring-assessment-attachment" aria-labelledby="authoring-assessment-attachment-title">
    <h4 id="authoring-assessment-attachment-title">Assessment</h4>
    <p v-if="currentAssessmentKey && currentKnown">An Assessment is selected.</p>
    <p v-else-if="currentAssessmentKey">Current Assessment reference is not available in this Draft’s Assessments.</p>
    <p v-if="loading" role="status">Loading Assessments…</p>
    <p v-else-if="error" role="alert">{{ error }} <BtgButton variant="secondary" @click="load(0)">Retry</BtgButton></p>
    <p v-else-if="!page.length">No Assessments yet. Create one in the Assessments section.</p>
    <ul v-else aria-label="Available Assessments"><li v-for="assessment in page" :key="assessment.assessmentKey"><BtgButton variant="secondary" :aria-pressed="assessment.assessmentKey === currentAssessmentKey" @click="select(assessment)">Use {{ assessment.title }}</BtgButton><span>{{ assessment.questionCount }} questions · updated <time :datetime="assessment.updatedAt">{{ new Date(assessment.updatedAt).toLocaleDateString() }}</time></span></li></ul>
    <p v-if="total > limit"><BtgButton variant="secondary" :disabled="loading || offset === 0" @click="load(Math.max(0, offset - limit))">Previous</BtgButton><BtgButton variant="secondary" :disabled="loading || offset + limit >= total" @click="load(offset + limit)">Next</BtgButton></p>
  </section>
</template>
