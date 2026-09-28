<template>
  <section class="authoring-lesson-preview" aria-labelledby="authoring-lesson-preview-title">
    <div v-if="state.kind === 'loading'" role="status"><h2 id="authoring-lesson-preview-title">Loading draft preview…</h2></div>
    <div v-else-if="state.kind === 'unavailable'" class="authoring-lesson-preview__state"><h2 id="authoring-lesson-preview-title">Draft preview unavailable</h2><p>We couldn’t load this saved Lesson content.</p><BtgButton variant="secondary" @click="load">Try again</BtgButton></div>
    <template v-else>
      <header class="authoring-lesson-preview__header">
        <RouterLink :to="editingPath">Back to editing</RouterLink>
        <p class="authoring-shell__eyebrow">Draft preview — not published</p>
        <p>This preview reflects your last saved changes.</p>
      </header>
      <div class="authoring-lesson-preview__controls">
        <span id="authoring-preview-width-label">Preview width</span>
        <div role="radiogroup" aria-labelledby="authoring-preview-width-label" class="authoring-lesson-preview__widths">
          <label v-for="option in widths" :key="option.id" :class="{ 'is-selected': width === option.id }"><input v-model="width" type="radio" :value="option.id" />{{ option.label }}</label>
        </div>
      </div>
      <div class="authoring-lesson-preview__workspace">
        <article class="authoring-lesson-preview__frame" :class="`authoring-lesson-preview__frame--${width}`" aria-label="Learner-facing lesson preview">
          <div class="lesson-page__content">
            <header class="lesson-page__header"><p class="lesson-page__eyebrow">{{ moduleTitle }}</p><h1 id="authoring-lesson-preview-title">{{ state.lesson.title }}</h1><p v-if="state.lesson.description" class="lesson-page__description">{{ state.lesson.description }}</p></header>
            <section v-if="state.lesson.objectives.length" aria-labelledby="authoring-preview-objectives"><h2 id="authoring-preview-objectives">Lesson objectives</h2><ul><li v-for="objective in state.lesson.objectives" :key="objective">{{ objective }}</li></ul></section>
            <section v-if="!state.content.blocks.length" class="lesson-content" aria-label="Lesson content"><p>This lesson has no published content yet.</p></section>
            <section v-else class="lesson-content" aria-label="Lesson content"><div v-for="block in state.content.blocks" :key="block.key" class="lesson-content__block"><LessonBlockRenderer :block="block" /></div></section>
          </div>
        </article>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { APIProblemError } from '../api/client'
import { getAuthoringLesson, getAuthoringStructure, InvalidAuthoringDraftIDError, type AuthoringLessonDetail } from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import BtgButton from '../components/BtgButton.vue'
import LessonBlockRenderer from '../components/LessonBlockRenderer.vue'
import { decodeLessonContent, type DecodedLessonContent } from '../lesson/content'

type Ready = { kind: 'ready'; lesson: AuthoringLessonDetail; content: Extract<DecodedLessonContent, { kind: 'ready' }> }
type State = { kind: 'loading' } | Ready | { kind: 'unavailable' }
const route = useRoute()
const { draft, markDraftUnavailable } = useAuthoringDraftContext()
const state = ref<State>({ kind: 'loading' })
const moduleTitle = ref('Lesson')
const width = ref<'desktop' | 'tablet' | 'mobile'>('desktop')
const widths = [{ id: 'desktop', label: 'Desktop' }, { id: 'tablet', label: 'Tablet' }, { id: 'mobile', label: 'Mobile' }] as const
const editingPath = computed(() => `/authoring/drafts/${encodeURIComponent(draft.value.id)}/lessons/${encodeURIComponent(lessonID())}/content`)
let active = true
let generation = 0
watch(() => route.params.lessonId, () => { void load() }, { immediate: true })
onBeforeUnmount(() => { active = false })
function lessonID() { return typeof route.params.lessonId === 'string' ? route.params.lessonId : '' }
async function load() {
  const current = ++generation
  state.value = { kind: 'loading' }
  try {
    const lesson = await getAuthoringLesson(draft.value.id, lessonID())
    if (!active || current !== generation) return
    const content = decodeLessonContent(lesson.content)
    if (content.kind !== 'ready') { state.value = { kind: 'unavailable' }; return }
    state.value = { kind: 'ready', lesson, content }
    void getAuthoringStructure(draft.value.id).then((structure) => { if (active && current === generation) moduleTitle.value = structure.modules.find((module) => module.id === lesson.module_id)?.title ?? 'Lesson' }).catch(() => {})
  } catch (error) {
    if (!active || current !== generation) return
    if (error instanceof InvalidAuthoringDraftIDError || (error instanceof APIProblemError && error.status === 404)) markDraftUnavailable()
    else state.value = { kind: 'unavailable' }
  }
}
</script>
