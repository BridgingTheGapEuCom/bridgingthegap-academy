<template>
  <BtgPageContainer as="section" class="translation-workspace" aria-labelledby="translation-title">
    <BtgPanel v-if="state === 'loading'" variant="subtle" role="status">
      <BtgPageTitle id="translation-title" :title="t('authoring.translations.context')" :description="t('authoring.translations.loadingWorkspace')" />
    </BtgPanel>
    <BtgPanel v-else-if="state === 'error'" variant="subtle" role="alert">
      <BtgPageTitle id="translation-title" :title="t('authoring.translations.workspaceUnavailableTitle')" :description="t('authoring.translations.workspaceUnavailableDescription')" />
      <BtgButton variant="secondary" @click="load">{{ t('authoring.translations.tryAgain') }}</BtgButton>
    </BtgPanel>
    <template v-else>
      <RouterLink class="btg-button btg-button--tertiary translation-workspace__back" :to="translationListPath">
        <BtgIcon name="arrowLeft" decorative />{{ t('authoring.translations.backToTranslations') }}
      </RouterLink>
      <BtgPageHeader
        title-id="translation-title"
        :eyebrow="t('authoring.translations.context')"
        :title="t('authoring.translations.workspaceTitle', { course: workspace.course.title.source, language: displayLanguage(workspace.targetLanguage) })"
        :description="t('authoring.translations.workspaceDescription')"
      >
        <template #metadata><BtgMetadataStrip class="translation-workspace__metadata" :items="workspaceMetadata" /></template>
      </BtgPageHeader>
      <BtgPanel variant="subtle" padding="compact"><p>{{ t('authoring.translations.appLocaleIndependent') }}</p></BtgPanel>
      <BtgPanel v-if="notice" variant="subtle" padding="compact" role="status" aria-live="polite"><p>{{ notice }}</p></BtgPanel>
      <BtgPanel v-if="conflict" class="translation-workspace__conflict" variant="subtle" role="alert">
        <h2>{{ t('authoring.translations.changedElsewhere') }}</h2>
        <p>{{ t('authoring.translations.reloadWarning') }}</p>
        <BtgButton variant="secondary" :loading="reloading" @click="load">{{ t('authoring.translations.reloadLatest') }}</BtgButton>
      </BtgPanel>

      <form class="translation-workspace__form" @submit.prevent="save">
        <BtgSection heading-id="translation-course-title">
          <template #heading>{{ t('authoring.translations.courseMetadata') }}</template>
          <p>{{ t('authoring.translations.courseMetadataDescription') }}</p>
          <div class="translation-workspace__fields">
            <TranslationField :reset-key="resetKey" :label="t('authoring.translations.courseTitle')" :copy="fieldCopy(t('authoring.translations.courseTitle'))" :field="workspace.course.title" @change="change({ target: 'COURSE', field: 'title' }, $event)" />
            <TranslationField :reset-key="resetKey" :label="t('authoring.translations.courseDescription')" :copy="fieldCopy(t('authoring.translations.courseDescription'))" :field="workspace.course.description" multiline @change="change({ target: 'COURSE', field: 'description' }, $event)" />
            <TranslationField v-for="(objective, objectiveIndex) in workspace.course.learningObjectives" :key="`course-objective-${objectiveIndex}`" :reset-key="resetKey" :label="t('authoring.translations.courseObjective', { position: objectiveIndex + 1 })" :copy="fieldCopy(t('authoring.translations.courseObjective', { position: objectiveIndex + 1 }))" :field="objective" multiline @change="change({ target: 'COURSE', field: 'objective', objective: objectiveIndex }, $event)" />
          </div>
        </BtgSection>

        <BtgSection v-for="(module, moduleIndex) in workspace.modules" :key="module.sourceStableKey" :heading-id="`translation-module-${moduleIndex}`">
          <template #heading>{{ t('authoring.translations.module', { position: moduleIndex + 1 }) }}: {{ module.title.source }}</template>
          <BtgBadge>{{ t('authoring.translations.moduleProgress', { translated: module.completeness.translatedFields, total: module.completeness.totalTranslatableFields }) }}</BtgBadge>
          <div class="translation-workspace__fields">
            <TranslationField :reset-key="resetKey" :label="t('authoring.translations.moduleTitle')" :copy="fieldCopy(t('authoring.translations.moduleTitle'))" :field="module.title" @change="change({ target: 'MODULE', moduleKey: module.sourceStableKey, field: 'title' }, $event)" />
            <TranslationField :reset-key="resetKey" :label="t('authoring.translations.moduleDescription')" :copy="fieldCopy(t('authoring.translations.moduleDescription'))" :field="module.description" multiline @change="change({ target: 'MODULE', moduleKey: module.sourceStableKey, field: 'description' }, $event)" />
          </div>
          <BtgPanel v-for="(lesson, lessonIndex) in module.lessons" :key="lesson.sourceStableKey" as="section" class="translation-workspace__lesson" variant="subtle">
            <header class="translation-workspace__section-header">
              <div><h3>{{ t('authoring.translations.lesson', { position: lessonIndex + 1 }) }}: {{ lesson.title.source }}</h3><p>{{ t('authoring.translations.lessonProgress', { translated: lesson.completeness.translatedFields, total: lesson.completeness.totalTranslatableFields }) }}</p></div>
            </header>
            <div class="translation-workspace__fields">
              <TranslationField :reset-key="resetKey" :label="t('authoring.translations.lessonTitle')" :copy="fieldCopy(t('authoring.translations.lessonTitle'))" :field="lesson.title" @change="change({ target: 'LESSON', moduleKey: module.sourceStableKey, lessonKey: lesson.sourceStableKey, field: 'title' }, $event)" />
              <TranslationField :reset-key="resetKey" :label="t('authoring.translations.lessonDescription')" :copy="fieldCopy(t('authoring.translations.lessonDescription'))" :field="lesson.description" multiline @change="change({ target: 'LESSON', moduleKey: module.sourceStableKey, lessonKey: lesson.sourceStableKey, field: 'description' }, $event)" />
              <TranslationField v-for="(objective, objectiveIndex) in lesson.learningObjectives" :key="`lesson-objective-${objectiveIndex}`" :reset-key="resetKey" :label="t('authoring.translations.lessonObjective', { position: objectiveIndex + 1 })" :copy="fieldCopy(t('authoring.translations.lessonObjective', { position: objectiveIndex + 1 }))" :field="objective" multiline @change="change({ target: 'LESSON', moduleKey: module.sourceStableKey, lessonKey: lesson.sourceStableKey, field: 'objective', objective: objectiveIndex }, $event)" />
            </div>
            <BtgPanel v-for="(block, blockIndex) in lesson.blocks" :key="block.sourceBlockKey" class="translation-workspace__block" padding="compact">
              <h4>{{ t('authoring.translations.contentBlock', { type: blockTypeLabel(block.type), position: blockIndex + 1 }) }}</h4>
              <p v-if="block.code" class="translation-workspace__readonly-context"><strong>{{ t('authoring.translations.codeReadOnly') }}</strong><code>{{ block.code }}</code></p>
              <p v-if="block.assessmentKey" class="translation-workspace__readonly-context">{{ t('authoring.translations.assessmentReference') }}</p>
              <TranslationField v-for="(field, name) in block.fields" :key="name" :reset-key="resetKey" :label="blockFieldLabel(name)" :copy="fieldCopy(blockFieldLabel(name))" :field="field" multiline @change="change({ target: 'BLOCK', moduleKey: module.sourceStableKey, lessonKey: lesson.sourceStableKey, blockKey: block.sourceBlockKey, field: name }, $event)" />
            </BtgPanel>
          </BtgPanel>
        </BtgSection>

        <BtgSection v-if="workspace.assessments.length" heading-id="translation-assessments-title">
          <template #heading>{{ t('authoring.translations.assessments') }}</template>
          <p>{{ t('authoring.translations.assessmentsDescription') }}</p>
          <BtgPanel v-for="(assessment, assessmentIndex) in workspace.assessments" :key="assessment.assessmentKey" class="translation-workspace__assessment" variant="subtle">
            <h3>{{ t('authoring.translations.assessment', { position: assessmentIndex + 1 }) }}</h3>
            <div v-for="(question, questionIndex) in assessment.questions" :key="question.sourceStableKey" class="translation-workspace__question">
              <h4>{{ t('authoring.translations.question', { position: questionIndex + 1 }) }}</h4>
              <TranslationField :reset-key="resetKey" :label="t('authoring.translations.questionPrompt')" :copy="fieldCopy(t('authoring.translations.questionPrompt'))" :field="question.prompt" multiline @change="change({ target: 'ASSESSMENT_QUESTION', assessmentKey: assessment.assessmentKey, questionKey: question.sourceStableKey, field: 'prompt' }, $event)" />
              <TranslationField v-for="(option, optionIndex) in question.options" :key="option.sourceStableKey" :reset-key="resetKey" :label="t('authoring.translations.option', { position: optionIndex + 1 })" :copy="fieldCopy(t('authoring.translations.option', { position: optionIndex + 1 }))" :field="option.text" @change="change({ target: 'ASSESSMENT_OPTION', assessmentKey: assessment.assessmentKey, questionKey: question.sourceStableKey, itemKey: option.sourceStableKey, field: 'text' }, $event)" />
              <TranslationField v-for="(item, itemIndex) in question.leftItems" :key="item.sourceStableKey" :reset-key="resetKey" :label="t('authoring.translations.leftItem', { position: itemIndex + 1 })" :copy="fieldCopy(t('authoring.translations.leftItem', { position: itemIndex + 1 }))" :field="item.text" @change="change({ target: 'ASSESSMENT_LEFT_ITEM', assessmentKey: assessment.assessmentKey, questionKey: question.sourceStableKey, itemKey: item.sourceStableKey, field: 'text' }, $event)" />
              <TranslationField v-for="(item, itemIndex) in question.rightItems" :key="item.sourceStableKey" :reset-key="resetKey" :label="t('authoring.translations.rightItem', { position: itemIndex + 1 })" :copy="fieldCopy(t('authoring.translations.rightItem', { position: itemIndex + 1 }))" :field="item.text" @change="change({ target: 'ASSESSMENT_RIGHT_ITEM', assessmentKey: assessment.assessmentKey, questionKey: question.sourceStableKey, itemKey: item.sourceStableKey, field: 'text' }, $event)" />
            </div>
          </BtgPanel>
        </BtgSection>

        <BtgSection heading-id="translation-publication-title">
          <template #heading>{{ t('authoring.translations.publication') }}</template>
          <p>{{ t('authoring.translations.publicationDescription') }}</p>
          <BtgPanel variant="subtle">
            <p v-if="!workspace.completeness.complete">{{ t('authoring.translations.incompletePublication') }}</p>
            <BtgButton type="button" :disabled="dirty || !workspace.completeness.complete || publishing" :loading="publishing" @click="publishOpen = true">
              {{ publishing ? t('authoring.translations.publishing') : t('authoring.translations.publishAction') }}
            </BtgButton>
          </BtgPanel>
        </BtgSection>

        <AuthoringDirtyActionBar :show="dirty" :busy="saving" :disabled="saving || reloading" :save-disabled="conflict" :message="t('authoring.translations.dirty')" :discard-label="t('authoring.translations.discard')" :save-label="t('authoring.translations.save')" :busy-label="t('authoring.translations.saving')" @discard="discard" />
      </form>

      <BtgDialog v-model:open="publishOpen" :show-trigger="false" :title="t('authoring.translations.publishDialogTitle')" :description="t('authoring.translations.publishDialogDescription', { language: displayLanguage(workspace.targetLanguage), version: workspace.source.version })">
        <template #footer>
          <BtgButton variant="secondary" @click="publishOpen = false">{{ t('authoring.translations.cancel') }}</BtgButton>
          <BtgButton :loading="publishing" @click="publish">{{ t('authoring.translations.confirmPublish') }}</BtgButton>
        </template>
      </BtgDialog>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { APIProblemError } from '../api/client'
import AuthoringDirtyActionBar from '../components/AuthoringDirtyActionBar.vue'
import BtgBadge from '../components/BtgBadge.vue'
import BtgButton from '../components/BtgButton.vue'
import BtgDialog from '../components/BtgDialog.vue'
import BtgIcon from '../components/BtgIcon.vue'
import BtgMetadataStrip from '../components/BtgMetadataStrip.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageHeader from '../components/BtgPageHeader.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import BtgSection from '../components/BtgSection.vue'
import TranslationField, { type TranslationFieldCopy } from '../components/TranslationField.vue'
import { languageLabel } from '../i18n/languages'
import { getTranslation, patchTranslation, publishTranslation, type TranslationChange, type TranslationWorkspace } from '../translations/translations'

const route = useRoute()
const { locale, t } = useI18n()
const workspace = ref<TranslationWorkspace>({} as TranslationWorkspace)
const state = ref<'loading' | 'ready' | 'error'>('loading')
const changes = ref<TranslationChange[]>([])
const saving = ref(false)
const reloading = ref(false)
const publishing = ref(false)
const conflict = ref(false)
const notice = ref('')
const resetKey = ref(0)
const publishOpen = ref(false)
let generation = 0
let active = true

const dirty = computed(() => changes.value.length > 0)
const translationListPath = computed(() => `/authoring/courses/${encodeURIComponent(workspace.value.source.courseId)}/versions/${encodeURIComponent(workspace.value.source.version)}/translations`)
const workspaceMetadata = computed(() => [
  { label: t('authoring.translations.sourceVersion'), value: workspace.value.source.version },
  { label: t('authoring.translations.sourceLanguage'), value: displayLanguage(workspace.value.source.language) },
  { label: t('authoring.translations.targetLanguage'), value: displayLanguage(workspace.value.targetLanguage) },
  { label: t('authoring.translations.statusLabel'), value: t(`authoring.translations.status.${workspace.value.lifecycle}`) },
  { label: t('authoring.translations.progressLabel'), value: progressLabel(workspace.value.completeness.translatedFields, workspace.value.completeness.totalTranslatableFields) },
])

watch(() => route.params.translationId, load, { immediate: true })
onBeforeUnmount(() => { active = false; generation += 1 })

function displayLanguage(value: string) { return languageLabel(value, locale.value) }
function progressLabel(translated: number, total: number) { return t('authoring.translations.fieldsProgress', { translated, total }, translated) }
function fieldCopy(label: string): TranslationFieldCopy { return {
  sourceRegion: `${label}: ${t('authoring.translations.sourceRegion')}`, targetRegion: `${label}: ${t('authoring.translations.targetRegion')}`, source: t('authoring.translations.source'), translation: t('authoring.translations.translation'), readOnly: t('authoring.translations.readOnly'), untranslated: t('authoring.translations.untranslated'), translated: t('authoring.translations.translated'), markUntranslated: t('authoring.translations.markUntranslated'), inputLabel: t('authoring.translations.translationInput', { field: label }),
} }
function blockTypeLabel(type: string) { return t(`authoring.translations.blockTypes.${type}`) }
function blockFieldLabel(field: string) { return t(`authoring.translations.blockFields.${field}`) }
function targetKey(target: Record<string, unknown>) { return JSON.stringify(target) }

async function load() {
  const request = ++generation
  const initial = state.value !== 'ready'
  if (initial) state.value = 'loading'; else reloading.value = true
  try {
    const latest = await getTranslation(String(route.params.translationId || ''))
    if (!active || request !== generation) return
    workspace.value = latest
    changes.value = []
    conflict.value = false
    resetKey.value += 1
    state.value = 'ready'
  } catch {
    if (active && request === generation) state.value = 'error'
  } finally {
    if (active && request === generation) reloading.value = false
  }
}
function change(target: Record<string, unknown>, translated: string | null) {
  const key = targetKey(target)
  changes.value = [...changes.value.filter((candidate) => targetKey(Object.fromEntries(Object.entries(candidate).filter(([name]) => name !== 'translated'))) !== key), { ...target, translated } as TranslationChange]
  notice.value = ''
}
function discard() { changes.value = []; conflict.value = false; notice.value = ''; resetKey.value += 1 }
async function save() {
  if (!dirty.value || saving.value || conflict.value) return
  saving.value = true
  notice.value = ''
  try {
    await patchTranslation(workspace.value.translationId, workspace.value.revision, changes.value)
    await load()
    if (active) notice.value = t('authoring.translations.saved')
  } catch (error) {
    if (!active) return
    if (error instanceof APIProblemError && error.status === 409) conflict.value = true
    else notice.value = t('authoring.translations.saveUnavailable')
  } finally {
    if (active) saving.value = false
  }
}
async function publish() {
  if (!workspace.value.completeness.complete || dirty.value || publishing.value) return
  publishOpen.value = false
  publishing.value = true
  notice.value = ''
  try {
    await publishTranslation(workspace.value.translationId, workspace.value.revision)
    await load()
    if (active) notice.value = t('authoring.translations.published')
  } catch (error) {
    if (!active) return
    if (error instanceof APIProblemError && error.status === 409) conflict.value = true
    else notice.value = t('authoring.translations.publishUnavailable')
  } finally {
    if (active) publishing.value = false
  }
}
</script>
