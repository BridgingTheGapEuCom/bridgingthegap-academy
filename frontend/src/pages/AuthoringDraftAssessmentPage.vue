<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { APIProblemError } from '../api/client'
import { assessmentFingerprint, moveInOrder, newMatchingItem, newOption, newQuestion, normalizeAssessmentQuestions, type AssessmentQuestionType } from '../authoring/assessments'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { authoringDraftAssessmentsPath, getAuthoringAssessment, replaceAuthoringAssessment, type AuthoringAssessmentDetail, type AuthoringAssessmentQuestion } from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import BtgButton from '../components/BtgButton.vue'
import BtgCheckbox from '../components/BtgCheckbox.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgPanel from '../components/BtgPanel.vue'
import BtgRadio from '../components/BtgRadio.vue'
import BtgSelect from '../components/BtgSelect.vue'
import BtgTextInput from '../components/BtgTextInput.vue'
import BtgTextarea from '../components/BtgTextarea.vue'
import AuthoringPageTitle from '../components/AuthoringPageTitle.vue'
import AuthoringSection from '../components/AuthoringSection.vue'

type Form = { title: string; questions: AuthoringAssessmentQuestion[] }
type State = { kind: 'loading' } | { kind: 'ready'; assessment: AuthoringAssessmentDetail } | { kind: 'unavailable' }
type AssessmentValidationCode = 'title_required' | 'question_prompt_required' | 'option_text_required' | 'single_correct_required' | 'multiple_correct_required' | 'matching_item_required'
const route = useRoute()
const { draft, markDraftUnavailable } = useAuthoringDraftContext()
const { t } = useI18n()
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

function copyQuestions(questions: AuthoringAssessmentQuestion[]): AuthoringAssessmentQuestion[] { return JSON.parse(JSON.stringify(questions)) as AuthoringAssessmentQuestion[] }
function copyForm(assessment: AuthoringAssessmentDetail): Form { return { title: assessment.title, questions: copyQuestions(assessment.questions) } }
function commit(assessment: AuthoringAssessmentDetail) { state.value = { kind: 'ready', assessment }; form.value = copyForm(assessment); baseline.value = assessmentFingerprint(form.value); conflict.value = false; error.value = undefined }
function typeLabel(type: AssessmentQuestionType) { return t(`authoring.assessments.types.${type}`) }
function validationMessage(code: AssessmentValidationCode) { return t(`authoring.assessments.validation.${code}`) }
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
function localError(): AssessmentValidationCode | undefined {
  if (!form.value.title.trim()) return 'title_required'
  for (const question of form.value.questions) {
    if (!question.prompt.trim()) return 'question_prompt_required'
    if ((question.type === 'SINGLE_CHOICE' || question.type === 'MULTIPLE_CHOICE') && question.options.some((option) => !option.text.trim())) return 'option_text_required'
    if (question.type === 'SINGLE_CHOICE' && question.correctOptionKeys.length !== 1) return 'single_correct_required'
    if (question.type === 'MULTIPLE_CHOICE' && !question.correctOptionKeys.length) return 'multiple_correct_required'
    if (question.type === 'MATCHING' && (question.leftItems.some((item) => !item.text.trim()) || question.rightItems.some((item) => !item.text.trim()))) return 'matching_item_required'
  }
}
async function save() {
  const isCurrent = captureScope()
  if (saving.value || reloading.value || conflict.value || state.value.kind !== 'ready' || !dirty.value) return
  const validation = localError(); if (validation) { error.value = validationMessage(validation); return }
  saving.value = true; error.value = undefined; message.value = undefined
  try {
    const result = await replaceAuthoringAssessment(draft.value.id, state.value.assessment.assessmentKey, { expectedRevision: state.value.assessment.revision, ...normalized() })
    if (!isCurrent() || !active || state.value.kind !== 'ready') return
    commit(result); message.value = t('authoring.assessments.saved')
  } catch (cause) {
    if (!isCurrent() || !active) return
    if (cause instanceof APIProblemError && cause.status === 404) { markDraftUnavailable(); return }
    if (cause instanceof APIProblemError && cause.status === 409) { conflict.value = true; return }
    error.value = cause instanceof APIProblemError && cause.status === 400 ? t('authoring.assessments.saveInvalid') : t('authoring.assessments.saveUnavailable')
  } finally { if (isCurrent() && active) saving.value = false }
}
async function reloadLatest() {
  const isCurrent = captureScope(); if (saving.value || reloading.value) return
  reloading.value = true; error.value = undefined
  try { const latest = await getAuthoringAssessment(draft.value.id, assessmentID()); if (isCurrent() && active) commit(latest) }
  catch (cause) { if (!isCurrent() || !active) return; if (cause instanceof APIProblemError && cause.status === 404) markDraftUnavailable(); else error.value = t('authoring.assessments.reloadUnavailable') }
  finally { if (isCurrent() && active) reloading.value = false }
}
function changeQuestion(index: number, next: AuthoringAssessmentQuestion) { if (!saving.value && !reloading.value) { const questions = [...form.value.questions]; questions[index] = next; form.value = { ...form.value, questions: normalizeAssessmentQuestions(questions) }; message.value = undefined } }
function addQuestion(type: AssessmentQuestionType) { if (!saving.value && !reloading.value) form.value = { ...form.value, questions: normalizeAssessmentQuestions([...form.value.questions, newQuestion(type)]) } }
function removeQuestion(index: number) { if (!saving.value && !reloading.value) form.value = { ...form.value, questions: normalizeAssessmentQuestions(form.value.questions.filter((_, itemIndex) => itemIndex !== index)) } }
function moveQuestion(index: number, direction: -1 | 1) { if (!saving.value && !reloading.value) form.value = { ...form.value, questions: normalizeAssessmentQuestions(moveInOrder(form.value.questions, index, direction)) } }
function updatePrompt(index: number, prompt: string) { const question = form.value.questions[index]!; changeQuestion(index, { ...question, prompt }) }
function updateOption(index: number, optionIndex: number, text: string) { const question = form.value.questions[index]!; const options = [...question.options]; options[optionIndex] = { ...options[optionIndex]!, text }; changeQuestion(index, { ...question, options }) }
function addOption(index: number) { const question = form.value.questions[index]!; changeQuestion(index, { ...question, options: [...question.options, newOption(question.options.length)] }) }
function removeOption(index: number, optionIndex: number) { const question = form.value.questions[index]!; if (question.options.length <= 2) return; const removed = question.options[optionIndex]!; const options = question.options.filter((_, itemIndex) => itemIndex !== optionIndex); changeQuestion(index, { ...question, options, correctOptionKeys: question.correctOptionKeys.filter((key) => key !== removed.stableKey) }) }
function moveOption(index: number, optionIndex: number, direction: -1 | 1) { const question = form.value.questions[index]!; changeQuestion(index, { ...question, options: moveInOrder(question.options, optionIndex, direction) }) }
function setSingleCorrect(index: number, key: string) { const question = form.value.questions[index]!; changeQuestion(index, { ...question, correctOptionKeys: [key] }) }
function toggleMultipleCorrect(index: number, key: string, checked: boolean) { const question = form.value.questions[index]!; changeQuestion(index, { ...question, correctOptionKeys: checked ? [...question.correctOptionKeys, key] : question.correctOptionKeys.filter((value) => value !== key) }) }
function updateMatchingItem(index: number, side: 'leftItems' | 'rightItems', itemIndex: number, text: string) { const question = form.value.questions[index]!; const items = [...question[side]]; items[itemIndex] = { ...items[itemIndex]!, text }; changeQuestion(index, { ...question, [side]: items }) }
function addMatchingPair(index: number) { const question = form.value.questions[index]!; const left = newMatchingItem('left', question.leftItems.length); const right = newMatchingItem('right', question.rightItems.length); changeQuestion(index, { ...question, leftItems: [...question.leftItems, left], rightItems: [...question.rightItems, right], correctPairs: [...question.correctPairs, { leftKey: left.stableKey, rightKey: right.stableKey }] }) }
function removeMatchingPair(index: number, itemIndex: number) { const question = form.value.questions[index]!; if (question.leftItems.length <= 1) return; const left = question.leftItems[itemIndex]!; const right = question.rightItems[itemIndex]!; changeQuestion(index, { ...question, leftItems: question.leftItems.filter((_, i) => i !== itemIndex), rightItems: question.rightItems.filter((_, i) => i !== itemIndex), correctPairs: question.correctPairs.filter((pair) => pair.leftKey !== left.stableKey && pair.rightKey !== right.stableKey) }) }
function moveMatchingPair(index: number, itemIndex: number, direction: -1 | 1) { const question = form.value.questions[index]!; changeQuestion(index, { ...question, leftItems: moveInOrder(question.leftItems, itemIndex, direction), rightItems: moveInOrder(question.rightItems, itemIndex, direction) }) }
function setMatchingPair(index: number, leftKey: string, rightKey: string) { const question = form.value.questions[index]!; const pairs = question.correctPairs.map((pair) => pair.leftKey === leftKey ? { leftKey, rightKey } : pair.rightKey === rightKey ? { ...pair, rightKey: question.correctPairs.find((current) => current.leftKey === leftKey)?.rightKey ?? pair.rightKey } : pair); changeQuestion(index, { ...question, correctPairs: pairs }) }
function selectedRight(question: AuthoringAssessmentQuestion, leftKey: string) { return question.correctPairs.find((pair) => pair.leftKey === leftKey)?.rightKey ?? '' }
function availableRight(question: AuthoringAssessmentQuestion, leftKey: string) { const current = selectedRight(question, leftKey); const used = new Set(question.correctPairs.filter((pair) => pair.leftKey !== leftKey).map((pair) => pair.rightKey)); return question.rightItems.filter((item) => item.stableKey === current || !used.has(item.stableKey)) }
</script>

<template>
  <section class="authoring-section authoring-assessment-editor" aria-labelledby="authoring-assessment-title">
    <RouterLink :to="authoringDraftAssessmentsPath(draft.id)">{{ t('authoring.assessments.back') }}</RouterLink>
    <BtgPanel v-if="state.kind === 'loading'" variant="subtle" padding="compact"><p role="status">{{ t('authoring.assessments.loading') }}</p></BtgPanel>
    <BtgPanel v-else-if="state.kind === 'unavailable'" variant="subtle" padding="compact"><AuthoringPageTitle id="authoring-assessment-title" :title="t('authoring.assessments.assessmentUnavailable')" :description="t('authoring.assessments.assessmentUnavailableDescription')" /><BtgButton variant="secondary" @click="load">{{ t('authoring.assessments.tryAgain') }}</BtgButton></BtgPanel>
    <form v-else class="authoring-assessment-editor__form" :aria-busy="saving" novalidate @submit.prevent="save">
      <AuthoringPageTitle id="authoring-assessment-title" :title="t('authoring.assessments.editTitle')" :description="t('authoring.assessments.editorDescription')" />
      <BtgPanel v-if="error" variant="subtle" padding="compact"><p role="alert">{{ error }}</p></BtgPanel><BtgPanel v-if="message" variant="subtle" padding="compact"><p role="status">{{ message }}</p></BtgPanel>
      <BtgPanel v-if="conflict" variant="subtle" padding="compact" role="status"><p>{{ t('authoring.assessments.changedElsewhere') }}</p><BtgButton variant="secondary" :disabled="reloading" @click="reloadLatest">{{ reloading ? t('authoring.assessments.reloading') : t('authoring.assessments.reloadLatest') }}</BtgButton></BtgPanel>
      <fieldset :disabled="saving || reloading || conflict">
        <BtgFormField :label="t('authoring.assessments.assessmentTitle')" required v-slot="{ controlId, describedBy }"><BtgTextInput :id="controlId" v-model="form.title" :aria-describedby="describedBy" maxlength="240" required /></BtgFormField>
        <AuthoringSection heading-id="authoring-assessment-add-question" :title="t('authoring.assessments.addQuestion')"><div class="authoring-assessment-editor__add"><BtgButton variant="secondary" leading-icon="add" @click="addQuestion('SINGLE_CHOICE')">{{ t('authoring.assessments.addSingle') }}</BtgButton><BtgButton variant="secondary" leading-icon="add" @click="addQuestion('MULTIPLE_CHOICE')">{{ t('authoring.assessments.addMultiple') }}</BtgButton><BtgButton variant="secondary" leading-icon="add" @click="addQuestion('MATCHING')">{{ t('authoring.assessments.addMatching') }}</BtgButton></div></AuthoringSection>
        <ol v-if="form.questions.length" class="authoring-assessment-editor__questions"><li v-for="(question, index) in form.questions" :key="question.stableKey"><BtgPanel padding="compact"><fieldset><legend>{{ t('authoring.assessments.question', { position: index + 1 }) }} · {{ typeLabel(question.type) }}</legend><BtgFormField :label="t('authoring.assessments.questionPrompt', { position: index + 1 })" required v-slot="{ controlId, describedBy }"><BtgTextarea :id="controlId" :model-value="question.prompt" :aria-describedby="describedBy" maxlength="10000" required @update:model-value="updatePrompt(index, $event)" /></BtgFormField>
          <fieldset v-if="question.type === 'SINGLE_CHOICE' || question.type === 'MULTIPLE_CHOICE'"><legend>{{ question.type === 'SINGLE_CHOICE' ? t('authoring.assessments.chooseOne') : t('authoring.assessments.chooseAll') }}</legend><ol><li v-for="(option, optionIndex) in question.options" :key="option.stableKey"><BtgRadio v-if="question.type === 'SINGLE_CHOICE'" :id="`${question.stableKey}-${option.stableKey}`" :model-value="question.correctOptionKeys[0] ?? ''" :name="question.stableKey" :value="option.stableKey" :label="t('authoring.assessments.option', { position: optionIndex + 1 })" @update:model-value="setSingleCorrect(index, $event)" /><BtgCheckbox v-else :model-value="question.correctOptionKeys.includes(option.stableKey)" :label="t('authoring.assessments.option', { position: optionIndex + 1 })" @update:model-value="toggleMultipleCorrect(index, option.stableKey, $event)" /><BtgFormField :label="t('authoring.assessments.option', { position: optionIndex + 1 })" required v-slot="{ controlId, describedBy }"><BtgTextInput :id="controlId" :model-value="option.text" :aria-describedby="describedBy" maxlength="4000" required @update:model-value="updateOption(index, optionIndex, $event)" /></BtgFormField><BtgButton variant="tertiary" :disabled="optionIndex === 0" :aria-label="t('authoring.assessments.moveOptionUp', { position: optionIndex + 1 })" @click="moveOption(index, optionIndex, -1)">{{ t('authoring.assessments.moveUp') }}</BtgButton><BtgButton variant="tertiary" :disabled="optionIndex === question.options.length - 1" :aria-label="t('authoring.assessments.moveOptionDown', { position: optionIndex + 1 })" @click="moveOption(index, optionIndex, 1)">{{ t('authoring.assessments.moveDown') }}</BtgButton><BtgButton variant="danger-secondary" :disabled="question.options.length <= 2" :aria-label="t('authoring.assessments.removeOption', { position: optionIndex + 1 })" @click="removeOption(index, optionIndex)">{{ t('authoring.assessments.remove') }}</BtgButton></li></ol><BtgButton variant="secondary" leading-icon="add" @click="addOption(index)">{{ t('authoring.assessments.addOption') }}</BtgButton></fieldset>
          <fieldset v-else><legend>{{ t('authoring.assessments.matchingInstructions') }}</legend><ol><li v-for="(left, itemIndex) in question.leftItems" :key="left.stableKey"><BtgFormField :label="t('authoring.assessments.leftItem', { position: itemIndex + 1 })" required v-slot="{ controlId, describedBy }"><BtgTextInput :id="controlId" :model-value="left.text" :aria-describedby="describedBy" maxlength="4000" required @update:model-value="updateMatchingItem(index, 'leftItems', itemIndex, $event)" /></BtgFormField><BtgFormField :label="t('authoring.assessments.rightItem', { position: itemIndex + 1 })" required v-slot="{ controlId, describedBy }"><BtgTextInput :id="controlId" :model-value="question.rightItems[itemIndex]?.text ?? ''" :aria-describedby="describedBy" maxlength="4000" required @update:model-value="updateMatchingItem(index, 'rightItems', itemIndex, $event)" /></BtgFormField><BtgFormField :label="t('authoring.assessments.correctMatch', { position: itemIndex + 1 })" v-slot="{ controlId, describedBy }"><BtgSelect :id="controlId" :model-value="selectedRight(question, left.stableKey)" :aria-describedby="describedBy" @update:model-value="setMatchingPair(index, left.stableKey, $event)"><option v-for="right in availableRight(question, left.stableKey)" :key="right.stableKey" :value="right.stableKey">{{ right.text || t('authoring.assessments.rightItemFallback', { position: right.position + 1 }) }}</option></BtgSelect></BtgFormField><BtgButton variant="tertiary" :disabled="itemIndex === 0" :aria-label="t('authoring.assessments.moveMatchingUp', { position: itemIndex + 1 })" @click="moveMatchingPair(index, itemIndex, -1)">{{ t('authoring.assessments.moveUp') }}</BtgButton><BtgButton variant="tertiary" :disabled="itemIndex === question.leftItems.length - 1" :aria-label="t('authoring.assessments.moveMatchingDown', { position: itemIndex + 1 })" @click="moveMatchingPair(index, itemIndex, 1)">{{ t('authoring.assessments.moveDown') }}</BtgButton><BtgButton variant="danger-secondary" :disabled="question.leftItems.length <= 1" :aria-label="t('authoring.assessments.removeMatching', { position: itemIndex + 1 })" @click="removeMatchingPair(index, itemIndex)">{{ t('authoring.assessments.remove') }}</BtgButton></li></ol><BtgButton variant="secondary" leading-icon="add" @click="addMatchingPair(index)">{{ t('authoring.assessments.addMatchingPair') }}</BtgButton></fieldset>
          <div class="authoring-assessment-editor__actions"><BtgButton variant="tertiary" :disabled="index === 0" :aria-label="t('authoring.assessments.moveQuestionUp', { position: index + 1 })" @click="moveQuestion(index, -1)">{{ t('authoring.assessments.moveUp') }}</BtgButton><BtgButton variant="tertiary" :disabled="index === form.questions.length - 1" :aria-label="t('authoring.assessments.moveQuestionDown', { position: index + 1 })" @click="moveQuestion(index, 1)">{{ t('authoring.assessments.moveDown') }}</BtgButton><BtgButton variant="danger-secondary" :aria-label="t('authoring.assessments.removeQuestion', { position: index + 1 })" @click="removeQuestion(index)">{{ t('authoring.assessments.removeQuestion', { position: index + 1 }) }}</BtgButton></div></fieldset></BtgPanel></li></ol>
        <BtgPanel v-else variant="subtle" padding="compact"><p>{{ t('authoring.assessments.noQuestions') }}</p></BtgPanel>
      </fieldset>
      <BtgPanel v-if="dirty" variant="subtle" padding="compact"><p role="status">{{ t('authoring.assessments.unsaved') }}</p></BtgPanel><BtgButton type="submit" :disabled="!dirty || saving || reloading || conflict">{{ saving ? t('authoring.assessments.saving') : t('authoring.assessments.save') }}</BtgButton>
    </form>
  </section>
</template>
