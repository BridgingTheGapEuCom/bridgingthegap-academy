<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useAuth } from '../auth/auth'
import type { LearnerAssessmentAttemptResponse, LearnerAttemptError, LearnerAttemptSession } from '../courses/attempts'
import type { PublishedAssessmentLearnerView } from '../courses/courses'
import BtgBadge from './BtgBadge.vue'
import BtgButton from './BtgButton.vue'
import BtgCheckbox from './BtgCheckbox.vue'
import BtgPanel from './BtgPanel.vue'
import BtgRadio from './BtgRadio.vue'
import BtgSelect from './BtgSelect.vue'

export type LearnerKnowledgeCheckCopy = {
  title: string; practice: string; submitted: string; unsaved: string; saved: string; instruction: string
  chooseMatch: string; save: string; saving: string; submit: string; submitting: string; conflict: string; resultTitle: string
  error: (code: LearnerAttemptError) => string
  result: (correct: number, total: number, percentage: number) => string
}

const props = defineProps<{ assessment: PublishedAssessmentLearnerView; copy: LearnerKnowledgeCheckCopy; attemptSession?: LearnerAttemptSession; blockKey?: string }>()
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
  <BtgPanel as="section" variant="subtle" class="learner-knowledge-check" :aria-labelledby="regionID">
    <h3 :id="regionID">{{ copy.title }}</h3>
    <p v-if="!isAuthenticated" class="learner-knowledge-check__note" role="note">{{ copy.practice }}</p>
    <p v-else-if="isSubmitted" class="learner-knowledge-check__note" role="note"><BtgBadge variant="success">{{ copy.submitted }}</BtgBadge></p>
    <p v-else-if="attemptSession?.dirty.value" class="learner-knowledge-check__note" role="status">{{ copy.unsaved }}</p>
    <p v-else-if="attemptSession?.attempt.value" class="learner-knowledge-check__note" role="status">{{ copy.saved }}</p>
    <p v-else class="learner-knowledge-check__note" role="note">{{ copy.instruction }}</p>

    <fieldset v-for="(question, questionIndex) in orderedQuestions" :key="question.stableKey" :disabled="isSubmitted || Boolean(pending)">
      <legend>{{ questionIndex + 1 }}. {{ question.prompt }}</legend>
      <template v-if="question.type === 'SINGLE_CHOICE'">
        <BtgRadio v-for="option in question.options" :key="option.stableKey" :name="`assessment-${assessment.assessmentKey}-${question.stableKey}`" :value="option.stableKey" :label="option.text" :model-value="selectedSingle(question.stableKey)" :disabled="isSubmitted || Boolean(pending)" @update:model-value="selectSingle(question.stableKey, $event)" />
      </template>
      <template v-else-if="question.type === 'MULTIPLE_CHOICE'">
        <BtgCheckbox v-for="option in question.options" :key="option.stableKey" :label="option.text" :model-value="selectedMultiple(question.stableKey).includes(option.stableKey)" :disabled="isSubmitted || Boolean(pending)" @update:model-value="toggleMultiple(question.stableKey, option.stableKey, $event)" />
      </template>
      <template v-else>
        <div v-for="leftItem in question.leftItems" :key="leftItem.stableKey" class="learner-knowledge-check__match">
          <label :for="`assessment-${assessment.assessmentKey}-${question.stableKey}-${leftItem.stableKey}`">{{ leftItem.text }}</label>
          <BtgSelect :id="`assessment-${assessment.assessmentKey}-${question.stableKey}-${leftItem.stableKey}`" :model-value="matchingValue(question.stableKey, leftItem.stableKey)" :disabled="isSubmitted || Boolean(pending)" @update:model-value="setMatching(question.stableKey, leftItem.stableKey, $event)">
            <option value="">{{ copy.chooseMatch }}</option>
            <option v-for="rightItem in question.rightItems" :key="rightItem.stableKey" :value="rightItem.stableKey" :disabled="rightItemTaken(question.stableKey, leftItem.stableKey, rightItem.stableKey)">{{ rightItem.text }}</option>
          </BtgSelect>
        </div>
      </template>
    </fieldset>

    <div v-if="isAuthenticated && attemptSession" class="learner-knowledge-check__actions">
      <BtgButton variant="secondary" :disabled="!attemptSession.dirty.value || Boolean(pending) || isSubmitted" @click="save">{{ pending === 'save' ? copy.saving : copy.save }}</BtgButton>
      <BtgButton :disabled="Boolean(pending) || isSubmitted" @click="submit">{{ pending === 'submit' ? copy.submitting : copy.submit }}</BtgButton>
    </div>
    <p v-if="attemptSession?.error.value" class="btg-form-field__error" role="alert">{{ copy.error(attemptSession.error.value) }}</p>
    <p v-if="attemptSession?.conflict.value" class="btg-form-field__error" role="alert">{{ copy.conflict }}</p>
    <BtgPanel v-if="attemptSession?.attempt.value?.result" as="section" class="learner-knowledge-check__result" variant="default" :aria-labelledby="resultID">
      <h4 :id="resultID" ref="resultHeading" tabindex="-1">{{ copy.resultTitle }}</h4>
      <p>{{ copy.result(attemptSession.attempt.value.result.correctCount, attemptSession.attempt.value.result.totalCount, attemptSession.attempt.value.result.percentage) }}</p>
    </BtgPanel>
  </BtgPanel>
</template>

<style scoped>
.learner-knowledge-check { display: grid; gap: var(--btg-space-4); }
.learner-knowledge-check__note { margin: 0; }
fieldset { display: grid; gap: var(--btg-space-2); margin: 0; min-inline-size: 0; border: 0; padding: 0; }
legend { margin-block-end: var(--btg-space-2); font-weight: 600; max-inline-size: 100%; overflow-wrap: anywhere; }
.learner-knowledge-check__match { display: grid; gap: var(--btg-space-2); grid-template-columns: minmax(0, 1fr) minmax(10rem, 1fr); align-items: center; overflow-wrap: anywhere; }
.learner-knowledge-check__actions { display: flex; flex-wrap: wrap; gap: var(--btg-space-3); }
.learner-knowledge-check__result { outline: none; }
@media (max-width: 30rem) { .learner-knowledge-check__match { grid-template-columns: 1fr; } }
</style>
