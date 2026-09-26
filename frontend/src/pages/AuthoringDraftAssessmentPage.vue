<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { APIProblemError } from '../api/client'
import { assessmentFingerprint, moveInOrder, newMatchingItem, newOption, newQuestion, normalizeAssessmentQuestions, type AssessmentQuestionType } from '../authoring/assessments'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { authoringDraftAssessmentsPath, getAuthoringAssessment, replaceAuthoringAssessment, type AuthoringAssessmentDetail, type AuthoringAssessmentQuestion } from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'

type Form = { title: string; questions: AuthoringAssessmentQuestion[] }
type State = { kind: 'loading' } | { kind: 'ready'; assessment: AuthoringAssessmentDetail } | { kind: 'unavailable' }
const route = useRoute()
const { draft, markDraftUnavailable } = useAuthoringDraftContext()
const state = ref<State>({ kind: 'loading' })
const form = ref<Form>({ title: '', questions: [] })
const baseline = ref('')
const saving = ref(false); const reloading = ref(false); const conflict = ref(false)
const error = ref<string>(); const message = ref<string>()
let requestVersion = 0; let active = true
const assessmentID = () => typeof route.params.assessmentId === 'string' ? route.params.assessmentId : ''
const captureScope = useAuthoringAsyncScope(() => `${draft.value.id}/${assessmentID()}`)
const dirty = computed(() => baseline.value !== '' && assessmentFingerprint(form.value) !== baseline.value)

watch(() => route.params.assessmentId, () => { void load() }, { immediate: true })
onBeforeUnmount(() => { active = false })

// Assessment DTOs are plain JSON. JSON copying intentionally strips Vue's
// reactive proxies before a complete aggregate is sent to the API.
function copyQuestions(questions: AuthoringAssessmentQuestion[]): AuthoringAssessmentQuestion[] { return JSON.parse(JSON.stringify(questions)) as AuthoringAssessmentQuestion[] }
function copyForm(assessment: AuthoringAssessmentDetail): Form { return { title: assessment.title, questions: copyQuestions(assessment.questions) } }
function commit(assessment: AuthoringAssessmentDetail) { state.value = { kind: 'ready', assessment }; form.value = copyForm(assessment); baseline.value = assessmentFingerprint(form.value); conflict.value = false; error.value = undefined }
async function load() {
  const isCurrent = captureScope(); const generation = ++requestVersion
  state.value = { kind: 'loading' }; saving.value = false; reloading.value = false; conflict.value = false; message.value = undefined; error.value = undefined
  try {
    const assessment = await getAuthoringAssessment(draft.value.id, assessmentID())
    if (!isCurrent() || !active || generation !== requestVersion) return
    commit(assessment)
  } catch (cause) {
    if (!isCurrent() || !active || generation !== requestVersion) return
    if (cause instanceof APIProblemError && cause.status === 404) { markDraftUnavailable(); return }
    state.value = { kind: 'unavailable' }
  }
}

function normalized(): Form { return { title: form.value.title.trim(), questions: normalizeAssessmentQuestions(copyQuestions(form.value.questions)) } }
function localError(): string | undefined {
  if (!form.value.title.trim()) return 'Enter an Assessment title.'
  for (const question of form.value.questions) {
    if (!question.prompt.trim()) return 'Enter a prompt for every question.'
    if ((question.type === 'SINGLE_CHOICE' || question.type === 'MULTIPLE_CHOICE') && question.options.some((option) => !option.text.trim())) return 'Enter text for every choice option.'
    if (question.type === 'SINGLE_CHOICE' && question.correctOptionKeys.length !== 1) return 'Choose one correct answer for every single-choice question.'
    if (question.type === 'MULTIPLE_CHOICE' && !question.correctOptionKeys.length) return 'Choose at least one correct answer for every multiple-choice question.'
    if (question.type === 'MATCHING' && (question.leftItems.some((item) => !item.text.trim()) || question.rightItems.some((item) => !item.text.trim()))) return 'Enter text for every matching item.'
  }
}
async function save() {
  const isCurrent = captureScope()
  if (saving.value || reloading.value || conflict.value || state.value.kind !== 'ready' || !dirty.value) return
  const validation = localError(); if (validation) { error.value = validation; return }
  saving.value = true; error.value = undefined; message.value = undefined
  try {
    const result = await replaceAuthoringAssessment(draft.value.id, state.value.assessment.assessmentKey, { expectedRevision: state.value.assessment.revision, ...normalized() })
    if (!isCurrent() || !active || state.value.kind !== 'ready') return
    commit(result); message.value = 'Assessment saved.'
  } catch (cause) {
    if (!isCurrent() || !active) return
    if (cause instanceof APIProblemError && cause.status === 404) { markDraftUnavailable(); return }
    if (cause instanceof APIProblemError && cause.status === 409) { conflict.value = true; return }
    error.value = cause instanceof APIProblemError && cause.status === 400 ? 'We couldn’t save this Assessment. Check the questions and try again.' : 'We couldn’t save this Assessment right now. Please try again.'
  } finally { if (isCurrent() && active) saving.value = false }
}
async function reloadLatest() {
  const isCurrent = captureScope(); if (saving.value || reloading.value) return
  reloading.value = true; error.value = undefined
  try { const latest = await getAuthoringAssessment(draft.value.id, assessmentID()); if (isCurrent() && active) commit(latest) }
  catch (cause) { if (!isCurrent() || !active) return; if (cause instanceof APIProblemError && cause.status === 404) markDraftUnavailable(); else error.value = 'We couldn’t reload this Assessment right now. Please try again.' }
  finally { if (isCurrent() && active) reloading.value = false }
}
function changeQuestion(index: number, next: AuthoringAssessmentQuestion) { if (!saving.value && !reloading.value) { const questions = [...form.value.questions]; questions[index] = next; form.value = { ...form.value, questions: normalizeAssessmentQuestions(questions) }; message.value = undefined } }
function addQuestion(type: AssessmentQuestionType) { if (!saving.value && !reloading.value) form.value = { ...form.value, questions: normalizeAssessmentQuestions([...form.value.questions, newQuestion(type)]) } }
function removeQuestion(index: number) { if (!saving.value && !reloading.value) form.value = { ...form.value, questions: normalizeAssessmentQuestions(form.value.questions.filter((_, itemIndex) => itemIndex !== index)) } }
function moveQuestion(index: number, direction: -1 | 1) { if (!saving.value && !reloading.value) form.value = { ...form.value, questions: normalizeAssessmentQuestions(moveInOrder(form.value.questions, index, direction)) } }
function updatePrompt(index: number, event: Event) { const question = form.value.questions[index]!; changeQuestion(index, { ...question, prompt: (event.target as HTMLTextAreaElement).value }) }
function updateOption(index: number, optionIndex: number, event: Event) { const question = form.value.questions[index]!; const options = [...question.options]; options[optionIndex] = { ...options[optionIndex]!, text: (event.target as HTMLInputElement).value }; changeQuestion(index, { ...question, options }) }
function addOption(index: number) { const question = form.value.questions[index]!; changeQuestion(index, { ...question, options: [...question.options, newOption(question.options.length)] }) }
function removeOption(index: number, optionIndex: number) { const question = form.value.questions[index]!; if (question.options.length <= 2) return; const removed = question.options[optionIndex]!; const options = question.options.filter((_, itemIndex) => itemIndex !== optionIndex); changeQuestion(index, { ...question, options, correctOptionKeys: question.correctOptionKeys.filter((key) => key !== removed.stableKey) }) }
function moveOption(index: number, optionIndex: number, direction: -1 | 1) { const question = form.value.questions[index]!; changeQuestion(index, { ...question, options: moveInOrder(question.options, optionIndex, direction) }) }
function setSingleCorrect(index: number, key: string) { const question = form.value.questions[index]!; changeQuestion(index, { ...question, correctOptionKeys: [key] }) }
function toggleMultipleCorrect(index: number, key: string, checked: boolean) { const question = form.value.questions[index]!; changeQuestion(index, { ...question, correctOptionKeys: checked ? [...question.correctOptionKeys, key] : question.correctOptionKeys.filter((value) => value !== key) }) }
function updateMatchingItem(index: number, side: 'leftItems' | 'rightItems', itemIndex: number, event: Event) { const question = form.value.questions[index]!; const items = [...question[side]]; items[itemIndex] = { ...items[itemIndex]!, text: (event.target as HTMLInputElement).value }; changeQuestion(index, { ...question, [side]: items }) }
function addMatchingPair(index: number) { const question = form.value.questions[index]!; const left = newMatchingItem('left', question.leftItems.length); const right = newMatchingItem('right', question.rightItems.length); changeQuestion(index, { ...question, leftItems: [...question.leftItems, left], rightItems: [...question.rightItems, right], correctPairs: [...question.correctPairs, { leftKey: left.stableKey, rightKey: right.stableKey }] }) }
function removeMatchingPair(index: number, itemIndex: number) { const question = form.value.questions[index]!; if (question.leftItems.length <= 1) return; const left = question.leftItems[itemIndex]!; const right = question.rightItems[itemIndex]!; changeQuestion(index, { ...question, leftItems: question.leftItems.filter((_, i) => i !== itemIndex), rightItems: question.rightItems.filter((_, i) => i !== itemIndex), correctPairs: question.correctPairs.filter((pair) => pair.leftKey !== left.stableKey && pair.rightKey !== right.stableKey) }) }
function moveMatchingPair(index: number, itemIndex: number, direction: -1 | 1) { const question = form.value.questions[index]!; changeQuestion(index, { ...question, leftItems: moveInOrder(question.leftItems, itemIndex, direction), rightItems: moveInOrder(question.rightItems, itemIndex, direction) }) }
function setMatchingPair(index: number, leftKey: string, rightKey: string) { const question = form.value.questions[index]!; const pairs = question.correctPairs.map((pair) => pair.leftKey === leftKey ? { leftKey, rightKey } : pair.rightKey === rightKey ? { ...pair, rightKey: question.correctPairs.find((current) => current.leftKey === leftKey)?.rightKey ?? pair.rightKey } : pair); changeQuestion(index, { ...question, correctPairs: pairs }) }
function selectedRight(question: AuthoringAssessmentQuestion, leftKey: string) { return question.correctPairs.find((pair) => pair.leftKey === leftKey)?.rightKey ?? '' }
function availableRight(question: AuthoringAssessmentQuestion, leftKey: string) { const current = selectedRight(question, leftKey); const used = new Set(question.correctPairs.filter((pair) => pair.leftKey !== leftKey).map((pair) => pair.rightKey)); return question.rightItems.filter((item) => item.stableKey === current || !used.has(item.stableKey)) }
</script>

<template>
  <section class="authoring-section authoring-assessment-editor" aria-labelledby="authoring-assessment-title">
    <RouterLink :to="authoringDraftAssessmentsPath(draft.id)">Back to Assessments</RouterLink>
    <p v-if="state.kind === 'loading'" role="status">Loading Assessment…</p>
    <div v-else-if="state.kind === 'unavailable'"><h2 id="authoring-assessment-title">Assessment unavailable</h2><p>We couldn’t load this Assessment right now.</p><BtgButton variant="secondary" @click="load">Try again</BtgButton></div>
    <form v-else class="authoring-assessment-editor__form" :aria-busy="saving" novalidate @submit.prevent="save">
      <header><h2 id="authoring-assessment-title">Edit Assessment</h2><p class="authoring-section__intro">Correct answers are visible only in this private Authoring editor.</p></header>
      <p v-if="error" role="alert" class="authoring-assessment-editor__error">{{ error }}</p><p v-if="message" role="status" class="authoring-assessment-editor__status">{{ message }}</p>
      <div v-if="conflict" class="authoring-assessment-editor__conflict" role="status"><p>This Assessment changed elsewhere. Your unsaved edits are still here. Reload the latest Assessment before saving again.</p><BtgButton variant="secondary" :disabled="reloading" @click="reloadLatest">{{ reloading ? 'Reloading…' : 'Reload latest Assessment' }}</BtgButton></div>
      <fieldset :disabled="saving || reloading || conflict"><BtgFormField label="Assessment title" required v-slot="{ controlId }"><input :id="controlId" v-model="form.title" maxlength="240" required /></BtgFormField>
        <section class="authoring-assessment-editor__add" aria-labelledby="authoring-assessment-add-question"><h3 id="authoring-assessment-add-question">Add question</h3><BtgButton variant="secondary" @click="addQuestion('SINGLE_CHOICE')">Add single-choice question</BtgButton><BtgButton variant="secondary" @click="addQuestion('MULTIPLE_CHOICE')">Add multiple-choice question</BtgButton><BtgButton variant="secondary" @click="addQuestion('MATCHING')">Add matching question</BtgButton></section>
        <ol v-if="form.questions.length" class="authoring-assessment-editor__questions"><li v-for="(question, index) in form.questions" :key="question.stableKey"><fieldset><legend>Question {{ index + 1 }} · {{ question.type.replaceAll('_', ' ').toLowerCase() }}</legend><BtgFormField :label="`Question ${index + 1} prompt`" required v-slot="{ controlId }"><textarea :id="controlId" :value="question.prompt" maxlength="10000" required @input="updatePrompt(index, $event)" /></BtgFormField>
          <fieldset v-if="question.type === 'SINGLE_CHOICE' || question.type === 'MULTIPLE_CHOICE'"><legend>{{ question.type === 'SINGLE_CHOICE' ? 'Choose one correct answer' : 'Choose all correct answers' }}</legend><ol><li v-for="(option, optionIndex) in question.options" :key="option.stableKey"><input :id="`${question.stableKey}-${option.stableKey}`" :type="question.type === 'SINGLE_CHOICE' ? 'radio' : 'checkbox'" :name="question.stableKey" :checked="question.correctOptionKeys.includes(option.stableKey)" @change="question.type === 'SINGLE_CHOICE' ? setSingleCorrect(index, option.stableKey) : toggleMultipleCorrect(index, option.stableKey, ($event.target as HTMLInputElement).checked)" /><BtgFormField :label="`Option ${optionIndex + 1}`" required v-slot="{ controlId }"><input :id="controlId" :value="option.text" maxlength="4000" required @input="updateOption(index, optionIndex, $event)" /></BtgFormField><BtgButton variant="secondary" :disabled="optionIndex === 0" :aria-label="`Move option ${optionIndex + 1} up`" @click="moveOption(index, optionIndex, -1)">Move up</BtgButton><BtgButton variant="secondary" :disabled="optionIndex === question.options.length - 1" :aria-label="`Move option ${optionIndex + 1} down`" @click="moveOption(index, optionIndex, 1)">Move down</BtgButton><BtgButton variant="secondary" :disabled="question.options.length <= 2" :aria-label="`Remove option ${optionIndex + 1}`" @click="removeOption(index, optionIndex)">Remove</BtgButton></li></ol><BtgButton variant="secondary" @click="addOption(index)">Add option</BtgButton></fieldset>
          <fieldset v-else><legend>Match each left item to one right item</legend><ol><li v-for="(left, itemIndex) in question.leftItems" :key="left.stableKey"><BtgFormField :label="`Left item ${itemIndex + 1}`" required v-slot="{ controlId }"><input :id="controlId" :value="left.text" maxlength="4000" required @input="updateMatchingItem(index, 'leftItems', itemIndex, $event)" /></BtgFormField><BtgFormField :label="`Right item ${itemIndex + 1}`" required v-slot="{ controlId }"><input :id="controlId" :value="question.rightItems[itemIndex]?.text" maxlength="4000" required @input="updateMatchingItem(index, 'rightItems', itemIndex, $event)" /></BtgFormField><BtgFormField :label="`Correct match for left item ${itemIndex + 1}`" v-slot="{ controlId }"><select :id="controlId" :value="selectedRight(question, left.stableKey)" @change="setMatchingPair(index, left.stableKey, ($event.target as HTMLSelectElement).value)"><option v-for="right in availableRight(question, left.stableKey)" :key="right.stableKey" :value="right.stableKey">{{ right.text || `Right item ${right.position + 1}` }}</option></select></BtgFormField><BtgButton variant="secondary" :disabled="itemIndex === 0" :aria-label="`Move matching item ${itemIndex + 1} up`" @click="moveMatchingPair(index, itemIndex, -1)">Move up</BtgButton><BtgButton variant="secondary" :disabled="itemIndex === question.leftItems.length - 1" :aria-label="`Move matching item ${itemIndex + 1} down`" @click="moveMatchingPair(index, itemIndex, 1)">Move down</BtgButton><BtgButton variant="secondary" :disabled="question.leftItems.length <= 1" :aria-label="`Remove matching item ${itemIndex + 1}`" @click="removeMatchingPair(index, itemIndex)">Remove pair</BtgButton></li></ol><BtgButton variant="secondary" @click="addMatchingPair(index)">Add matching pair</BtgButton></fieldset>
          <div class="authoring-assessment-editor__actions"><BtgButton variant="secondary" :disabled="index === 0" :aria-label="`Move question ${index + 1} up`" @click="moveQuestion(index, -1)">Move up</BtgButton><BtgButton variant="secondary" :disabled="index === form.questions.length - 1" :aria-label="`Move question ${index + 1} down`" @click="moveQuestion(index, 1)">Move down</BtgButton><BtgButton variant="secondary" :aria-label="`Remove question ${index + 1}`" @click="removeQuestion(index)">Remove question</BtgButton></div></fieldset></li></ol>
        <p v-else>No questions yet. Add a question when you are ready.</p></fieldset>
      <p v-if="dirty" role="status">You have unsaved Assessment changes.</p><BtgButton type="submit" :disabled="!dirty || saving || reloading || conflict">{{ saving ? 'Saving Assessment…' : 'Save Assessment' }}</BtgButton>
    </form>
  </section>
</template>
