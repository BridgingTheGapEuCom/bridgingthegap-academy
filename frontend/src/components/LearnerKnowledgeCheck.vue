<script setup lang="ts">
import { computed, ref } from 'vue'
import type { PublishedAssessmentLearnerView } from '../courses/courses'

const props = defineProps<{ assessment: PublishedAssessmentLearnerView }>()

const singleChoiceResponses = ref<Record<string, string>>({})
const multipleChoiceResponses = ref<Record<string, string[]>>({})
const matchingResponses = ref<Record<string, string>>({})

const orderedQuestions = computed(() => [...props.assessment.questions].sort((a, b) => a.position - b.position))

function multipleChoiceSelected(questionKey: string, optionKey: string): boolean {
  return multipleChoiceResponses.value[questionKey]?.includes(optionKey) ?? false
}

function toggleMultipleChoice(questionKey: string, optionKey: string, selected: boolean) {
  const current = multipleChoiceResponses.value[questionKey] ?? []
  multipleChoiceResponses.value = {
    ...multipleChoiceResponses.value,
    [questionKey]: selected ? [...current, optionKey] : current.filter((value) => value !== optionKey),
  }
}

function rightItemTaken(questionKey: string, leftKey: string, rightKey: string): boolean {
  return Object.entries(matchingResponses.value)
    .some(([key, value]) => key !== `${questionKey}:${leftKey}` && value === rightKey)
}
</script>

<template>
  <section class="lesson-block lesson-knowledge-check" :aria-labelledby="`knowledge-check-${assessment.assessmentKey}`">
    <h3 :id="`knowledge-check-${assessment.assessmentKey}`" class="lesson-block__label">Knowledge check</h3>
    <p class="lesson-knowledge-check__note">Practice responses stay on this page and are not saved or graded yet.</p>

    <fieldset v-for="question in orderedQuestions" :key="question.stableKey" class="lesson-knowledge-check__question">
      <legend>{{ question.position + 1 }}. {{ question.prompt }}</legend>

      <template v-if="question.type === 'SINGLE_CHOICE'">
        <label v-for="option in question.options" :key="option.stableKey" class="lesson-knowledge-check__option">
          <input
            v-model="singleChoiceResponses[question.stableKey]"
            type="radio"
            :name="`${assessment.assessmentKey}-${question.stableKey}`"
            :value="option.stableKey"
          />
          {{ option.text }}
        </label>
      </template>

      <template v-else-if="question.type === 'MULTIPLE_CHOICE'">
        <label v-for="option in question.options" :key="option.stableKey" class="lesson-knowledge-check__option">
          <input
            type="checkbox"
            :checked="multipleChoiceSelected(question.stableKey, option.stableKey)"
            @change="toggleMultipleChoice(question.stableKey, option.stableKey, ($event.target as HTMLInputElement).checked)"
          />
          {{ option.text }}
        </label>
      </template>

      <div v-else class="lesson-knowledge-check__matching">
        <label v-for="leftItem in question.leftItems" :key="leftItem.stableKey" class="lesson-knowledge-check__match-row">
          <span>{{ leftItem.text }}</span>
          <select v-model="matchingResponses[`${question.stableKey}:${leftItem.stableKey}`]" :aria-label="`Match ${leftItem.text}`">
            <option value="">Choose a match</option>
            <option v-for="rightItem in question.rightItems" :key="rightItem.stableKey" :value="rightItem.stableKey" :disabled="rightItemTaken(question.stableKey, leftItem.stableKey, rightItem.stableKey)">
              {{ rightItem.text }}
            </option>
          </select>
        </label>
      </div>
    </fieldset>
  </section>
</template>

<style scoped>
.lesson-knowledge-check { border-inline-start: 0.25rem solid var(--color-border, #536878); padding-inline-start: 1rem; }
.lesson-knowledge-check__note { margin-block: 0.5rem 1rem; }
.lesson-knowledge-check__question { display: grid; gap: 0.5rem; margin: 1rem 0; min-inline-size: 0; }
.lesson-knowledge-check__question legend { font-weight: 600; max-inline-size: 100%; }
.lesson-knowledge-check__option { display: flex; align-items: baseline; gap: 0.5rem; overflow-wrap: anywhere; }
.lesson-knowledge-check__matching { display: grid; gap: 0.65rem; }
.lesson-knowledge-check__match-row { display: grid; gap: 0.35rem; grid-template-columns: minmax(0, 1fr) minmax(10rem, 1fr); align-items: center; overflow-wrap: anywhere; }
select { max-inline-size: 100%; min-inline-size: 0; }
@media (max-width: 30rem) { .lesson-knowledge-check__match-row { grid-template-columns: 1fr; } }
</style>
