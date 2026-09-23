<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useAuth } from '../auth/auth'
import type { LearnerAssessmentAttemptResponse, LearnerAttemptSession } from '../courses/attempts'
import type { PublishedAssessmentLearnerView } from '../courses/courses'

const props = defineProps<{ assessment: PublishedAssessmentLearnerView; attemptSession?: LearnerAttemptSession; blockKey?: string }>()
const auth = useAuth()
const localResponses = ref<LearnerAssessmentAttemptResponse[]>([])
const resultHeading = ref<HTMLElement | null>(null)
const responses = computed(() => props.attemptSession?.responses.value ?? localResponses.value)
const isAuthenticated = computed(() => auth.state.value.status === 'authenticated')
const isSubmitted = computed(() => props.attemptSession?.submitted.value ?? false)
const pending = computed(() => props.attemptSession?.pending.value)
const regionID = computed(() => `knowledge-check-${props.blockKey ?? props.assessment.assessmentKey}`)
const resultID = computed(() => `${regionID.value}-result`)
const orderedQuestions = computed(() => [...props.assessment.questions].sort((a, b) => a.position - b.position))

function responseFor(questionKey: string) { return responses.value.find((response) => response.questionKey === questionKey) }
function setResponse(next: LearnerAssessmentAttemptResponse) {
  if (isSubmitted.value) return
  const value = responses.value.filter((response) => response.questionKey !== next.questionKey)
  value.push(next)
  value.sort((a, b) => a.questionKey.localeCompare(b.questionKey))
  if (props.attemptSession) props.attemptSession.responses.value = value
  else localResponses.value = value
}
function selectedSingle(questionKey: string) { const response = responseFor(questionKey); return response?.type === 'SINGLE_CHOICE' ? response.selectedOptionKey : '' }
function selectSingle(questionKey: string, optionKey: string) { setResponse({ questionKey, type: 'SINGLE_CHOICE', selectedOptionKey: optionKey, selectedOptionKeys: [], pairs: [] }) }
function selectedMultiple(questionKey: string) { const response = responseFor(questionKey); return response?.type === 'MULTIPLE_CHOICE' ? response.selectedOptionKeys : [] }
function toggleMultiple(questionKey: string, optionKey: string, checked: boolean) {
  const keys = checked ? [...selectedMultiple(questionKey), optionKey] : selectedMultiple(questionKey).filter((key) => key !== optionKey)
  if (!keys.length) { const value = responses.value.filter((response) => response.questionKey !== questionKey); if (props.attemptSession) props.attemptSession.responses.value = value; else localResponses.value = value; return }
  setResponse({ questionKey, type: 'MULTIPLE_CHOICE', selectedOptionKey: '', selectedOptionKeys: [...new Set(keys)].sort(), pairs: [] })
}
function matchingValue(questionKey: string, leftItemKey: string) { const response = responseFor(questionKey); return response?.type === 'MATCHING' ? response.pairs.find((pair) => pair.leftItemKey === leftItemKey)?.rightItemKey ?? '' : '' }
function setMatching(questionKey: string, leftItemKey: string, rightItemKey: string) {
  const response = responseFor(questionKey)
  const pairs = response?.type === 'MATCHING' ? response.pairs.filter((pair) => pair.leftItemKey !== leftItemKey && pair.rightItemKey !== rightItemKey) : []
  if (rightItemKey) pairs.push({ leftItemKey, rightItemKey })
  if (!pairs.length) { const value = responses.value.filter((item) => item.questionKey !== questionKey); if (props.attemptSession) props.attemptSession.responses.value = value; else localResponses.value = value; return }
  pairs.sort((a, b) => a.leftItemKey.localeCompare(b.leftItemKey))
  setResponse({ questionKey, type: 'MATCHING', selectedOptionKey: '', selectedOptionKeys: [], pairs })
}
function rightItemTaken(questionKey: string, leftItemKey: string, rightItemKey: string) { return responses.value.find((response) => response.questionKey === questionKey && response.type === 'MATCHING')?.pairs.some((pair) => pair.leftItemKey !== leftItemKey && pair.rightItemKey === rightItemKey) ?? false }
async function save() { await props.attemptSession?.save() }
async function submit() { if (await props.attemptSession?.submit()) { await nextTick(); resultHeading.value?.focus() } }
</script>

<template>
  <section class="lesson-block learner-knowledge-check" :aria-labelledby="regionID">
    <h3 :id="regionID">Knowledge check</h3>
    <p v-if="!isAuthenticated" class="learner-knowledge-check__note" role="note">Practice responses stay on this page. Sign in to save and submit your answers.</p>
    <p v-else-if="isSubmitted" class="learner-knowledge-check__note" role="note">This attempt has been submitted.</p>
    <p v-else-if="attemptSession?.dirty.value" class="learner-knowledge-check__note" role="status">Unsaved changes.</p>
    <p v-else-if="attemptSession?.attempt.value" class="learner-knowledge-check__note" role="status">Progress saved.</p>
    <p v-else class="learner-knowledge-check__note" role="note">Choose answers, then save progress when you are ready.</p>

    <fieldset v-for="(question, questionIndex) in orderedQuestions" :key="question.stableKey" :disabled="isSubmitted || Boolean(pending)">
      <legend>{{ questionIndex + 1 }}. {{ question.prompt }}</legend>
      <template v-if="question.type === 'SINGLE_CHOICE'">
        <label v-for="option in question.options" :key="option.stableKey">
          <input type="radio" :name="`assessment-${assessment.assessmentKey}-${question.stableKey}`" :value="option.stableKey" :checked="selectedSingle(question.stableKey) === option.stableKey" @change="selectSingle(question.stableKey, option.stableKey)" />
          {{ option.text }}
        </label>
      </template>
      <template v-else-if="question.type === 'MULTIPLE_CHOICE'">
        <label v-for="option in question.options" :key="option.stableKey">
          <input type="checkbox" :checked="selectedMultiple(question.stableKey).includes(option.stableKey)" @change="toggleMultiple(question.stableKey, option.stableKey, ($event.target as HTMLInputElement).checked)" />
          {{ option.text }}
        </label>
      </template>
      <template v-else>
        <div v-for="leftItem in question.leftItems" :key="leftItem.stableKey" class="learner-knowledge-check__match">
          <label :for="`assessment-${assessment.assessmentKey}-${question.stableKey}-${leftItem.stableKey}`">{{ leftItem.text }}</label>
          <select :id="`assessment-${assessment.assessmentKey}-${question.stableKey}-${leftItem.stableKey}`" :value="matchingValue(question.stableKey, leftItem.stableKey)" @change="setMatching(question.stableKey, leftItem.stableKey, ($event.target as HTMLSelectElement).value)">
            <option value="">Choose a match</option>
            <option v-for="rightItem in question.rightItems" :key="rightItem.stableKey" :value="rightItem.stableKey" :disabled="rightItemTaken(question.stableKey, leftItem.stableKey, rightItem.stableKey)">{{ rightItem.text }}</option>
          </select>
        </div>
      </template>
    </fieldset>

    <div v-if="isAuthenticated && attemptSession" class="learner-knowledge-check__actions">
      <button type="button" :disabled="!attemptSession.dirty.value || Boolean(pending) || isSubmitted" @click="save">{{ pending === 'save' ? 'Saving progress…' : 'Save progress' }}</button>
      <button type="button" :disabled="Boolean(pending) || isSubmitted" @click="submit">{{ pending === 'submit' ? 'Submitting…' : 'Submit answers' }}</button>
    </div>
    <p v-if="attemptSession?.error.value" class="learner-knowledge-check__error" role="alert">{{ attemptSession.error.value }}</p>
    <p v-if="attemptSession?.conflict.value" class="learner-knowledge-check__error" role="alert">Progress changed elsewhere. The latest saved attempt was loaded; your local answers were not sent.</p>
    <section v-if="attemptSession?.attempt.value?.result" class="learner-knowledge-check__result" :aria-labelledby="resultID" tabindex="-1" ref="resultHeading">
      <h4 :id="resultID">Submitted result</h4>
      <p>{{ attemptSession.attempt.value.result.correctCount }} of {{ attemptSession.attempt.value.result.totalCount }} questions correct ({{ attemptSession.attempt.value.result.percentage }}%).</p>
    </section>
  </section>
</template>

<style scoped>
.learner-knowledge-check { border-inline-start: 0.25rem solid var(--color-border, #536878); padding-inline-start: 1rem; }
.learner-knowledge-check__note { margin-block: 0.5rem 1rem; }
fieldset { display: grid; gap: 0.5rem; margin: 1rem 0; min-inline-size: 0; }
legend { font-weight: 600; max-inline-size: 100%; }
fieldset label { display: flex; align-items: baseline; gap: 0.5rem; overflow-wrap: anywhere; }
.learner-knowledge-check__match { display: grid; gap: 0.35rem; grid-template-columns: minmax(0, 1fr) minmax(10rem, 1fr); align-items: center; overflow-wrap: anywhere; }
select { max-inline-size: 100%; min-inline-size: 0; }
.learner-knowledge-check__actions { display: flex; flex-wrap: wrap; gap: 0.75rem; margin-block: 1rem; }
.learner-knowledge-check__error { margin-block: 0.75rem; }
.learner-knowledge-check__result { margin-block: 1rem; }
@media (max-width: 30rem) { .learner-knowledge-check__match { grid-template-columns: 1fr; } }
</style>
