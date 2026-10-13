<template>
  <BtgPageContainer as="section" class="course-import" aria-labelledby="course-import-title">
    <p class="sr-only" role="status" aria-live="polite">{{ announcement }}</p>
    <p class="btg-page-eyebrow">{{ t('admin.context') }}</p>
    <BtgPageTitle id="course-import-title" :title="t('admin.import.title')" :description="t('admin.import.helper')" />

    <BtgPanel v-if="state.kind === 'SELECT' || state.kind === 'VALIDATING' || state.kind === 'ERROR'" class="course-import__panel">
      <form class="course-import__form" @submit.prevent="validate">
        <BtgFileField ref="fileField" v-model="file" :label="t('admin.import.fileLabel')" :description="t('admin.import.fileHelper')" :empty-text="t('admin.import.noFile')" :remove-label="t('admin.import.removeFile')" :selected-description="file ? bytes(file.size) : undefined" :error="state.kind === 'ERROR' ? state.message : undefined" accept=".zip,application/zip" required :disabled="state.kind === 'VALIDATING'" @update:model-value="selectFile" />
        <p v-if="state.kind === 'ERROR'" ref="errorSummary" class="sr-only" tabindex="-1">{{ state.message }}</p>
        <div class="course-import__actions">
          <BtgButton type="submit" leading-icon="import" :loading="state.kind === 'VALIDATING'" :disabled="!file || state.kind === 'VALIDATING'">{{ state.kind === 'VALIDATING' ? t('admin.import.validating') : t('admin.import.validate') }}</BtgButton>
        </div>
      </form>
    </BtgPanel>

    <BtgSection v-else-if="state.kind === 'PREVIEW' || state.kind === 'IMPORTING'" heading-id="course-import-preview-title">
      <template #heading><span ref="previewHeading" tabindex="-1">{{ t('admin.import.reviewTitle') }}</span></template>
      <div class="course-import__preview">
        <BtgPanel><h3>{{ t('admin.import.course') }}</h3><BtgMetadataStrip layout="two-column" :items="courseMetadata(state.preview)" /></BtgPanel>
        <BtgPanel><h3>{{ t('admin.import.contents') }}</h3><BtgMetadataStrip layout="two-column" :items="contentMetadata(state.preview)" /></BtgPanel>
        <BtgPanel><h3>{{ t('admin.import.translations') }}</h3><p>{{ state.preview.translationLanguages.length ? state.preview.translationLanguages.join(', ') : t('admin.import.noTranslations') }}</p></BtgPanel>
        <BtgPanel v-if="state.preview.attribution.length"><h3>{{ t('admin.import.attribution') }}</h3><ul><li v-for="person in state.preview.attribution" :key="`${person.order}:${person.displayName}`">{{ t('admin.import.attributionEntry', { name: person.displayName, role: person.role }) }}</li></ul></BtgPanel>
      </div>
      <BtgPanel variant="subtle"><p>{{ t('admin.import.notice') }}</p></BtgPanel>
      <p v-if="state.kind === 'IMPORTING'" role="status">{{ t('admin.import.importing') }}</p>
      <p v-if="state.kind === 'PREVIEW' && state.message" class="btg-form-field__error" role="alert">{{ state.message }}</p>
      <div class="course-import__actions">
        <BtgButton leading-icon="import" :loading="state.kind === 'IMPORTING'" :disabled="state.kind === 'IMPORTING'" @click="execute">{{ state.kind === 'IMPORTING' ? t('admin.import.importing') : t('admin.import.import') }}</BtgButton>
        <BtgButton variant="secondary" :disabled="state.kind === 'IMPORTING'" @click="reset">{{ t('admin.import.chooseAnother') }}</BtgButton>
      </div>
    </BtgSection>

    <BtgPanel v-else-if="state.kind === 'SUCCESS'" class="course-import__result" variant="subtle">
      <BtgPageTitle id="course-import-result-title" :title="state.result.status === 'REPLAYED' ? t('admin.import.replayedTitle') : t('admin.import.importedTitle')" :description="state.result.status === 'REPLAYED' ? t('admin.import.replayedDescription') : t('admin.import.importedDescription', { title: state.title, version: state.result.semVer })" />
      <p v-if="state.result.translationLanguages.length">{{ t('admin.import.translationsResult', { languages: state.result.translationLanguages.join(', ') }) }}</p>
      <div class="course-import__actions"><RouterLink class="btg-button btg-button--primary" :to="publishedCourseVersionPath(state.result.courseId, state.result.semVer)">{{ t('admin.import.viewCourse') }}</RouterLink><BtgButton variant="secondary" @click="reset">{{ t('admin.import.importAnother') }}</BtgButton></div>
    </BtgPanel>

    <BtgPanel v-else class="course-import__result" variant="subtle" role="alert">
      <BtgPageTitle id="course-import-conflict-title" :title="t('admin.import.conflictTitle')" :description="t('admin.import.conflictDescription')" />
      <BtgButton variant="secondary" @click="reset">{{ t('admin.import.importAnother') }}</BtgButton>
    </BtgPanel>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BtgButton from '../components/BtgButton.vue'
import BtgFileField from '../components/BtgFileField.vue'
import BtgMetadataStrip from '../components/BtgMetadataStrip.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import BtgSection from '../components/BtgSection.vue'
import { publishedCourseVersionPath } from '../courses/courses'
import { useApplicationFormatters } from '../i18n/formatters'
import { classifyCoursePackageFailure, coursePackageValidationError, executeCoursePackage, previewCoursePackage, type CoursePackageImportResult, type CoursePackagePreviewResponse } from '../portability/portability'

type Preview = CoursePackagePreviewResponse['preview']
type State = { kind: 'SELECT' } | { kind: 'VALIDATING' } | { kind: 'PREVIEW'; preview: Preview; token: string; message?: string } | { kind: 'IMPORTING'; preview: Preview; token: string } | { kind: 'SUCCESS'; title: string; result: CoursePackageImportResult } | { kind: 'CONFLICT' } | { kind: 'ERROR'; message: string }
const { t } = useI18n()
const { bytes, number } = useApplicationFormatters()
const state = ref<State>({ kind: 'SELECT' })
const file = ref<File>()
const fileField = ref<InstanceType<typeof BtgFileField>>()
const previewHeading = ref<HTMLElement>()
const errorSummary = ref<HTMLElement>()
const announcement = ref('')
let active = true
let generation = 0
onBeforeUnmount(() => { active = false; generation += 1 })

function selectFile(selected?: File) { generation += 1; file.value = selected; state.value = { kind: 'SELECT' }; announcement.value = selected ? t('admin.import.selected', { filename: selected.name }) : '' }
function courseMetadata(preview: Preview) { return [{ label: t('admin.import.titleLabel'), value: preview.title }, { label: t('admin.import.version'), value: preview.version }, { label: t('admin.import.sourceLanguage'), value: preview.language }, { label: t('admin.import.license'), value: preview.license.displayName }] }
function contentMetadata(preview: Preview) { return [{ label: t('admin.import.modules'), value: number(preview.moduleCount) }, { label: t('admin.import.lessons'), value: number(preview.lessonCount) }, { label: t('admin.import.assessments'), value: number(preview.assessmentCount) }, { label: t('admin.import.assets'), value: `${number(preview.assetCount)} · ${bytes(preview.assetBytes)}` }] }
function validationMessage(error: unknown) { return t(`admin.import.errors.${coursePackageValidationError(error)}`) }

async function validate() {
  if (!file.value || state.value.kind === 'VALIDATING') return
  const selected = file.value; const request = ++generation
  state.value = { kind: 'VALIDATING' }; announcement.value = t('admin.import.validating')
  try {
    const response = await previewCoursePackage(selected)
    if (!active || request !== generation || file.value !== selected) return
    state.value = { kind: 'PREVIEW', preview: response.preview, token: response.previewToken }; announcement.value = t('admin.import.validationComplete')
    await nextTick(); previewHeading.value?.focus()
  } catch (error) {
    if (!active || request !== generation || file.value !== selected) return
    state.value = { kind: 'ERROR', message: validationMessage(error) }; announcement.value = t('admin.import.validationFailed')
    await nextTick(); errorSummary.value?.focus()
  }
}
async function execute() {
  if (state.value.kind !== 'PREVIEW') return
  const snapshot = state.value; const request = ++generation
  state.value = { kind: 'IMPORTING', preview: snapshot.preview, token: snapshot.token }; announcement.value = t('admin.import.importingAnnouncement')
  try {
    const result = await executeCoursePackage(snapshot.token)
    if (!active || request !== generation) return
    state.value = { kind: 'SUCCESS', title: snapshot.preview.title, result }; announcement.value = result.status === 'REPLAYED' ? t('admin.import.replayedAnnouncement') : t('admin.import.importedAnnouncement')
  } catch (error) {
    if (!active || request !== generation) return
    const kind = classifyCoursePackageFailure(error)
    if (kind === 'expired') { state.value = { kind: 'ERROR', message: t('admin.import.previewExpired') }; announcement.value = t('admin.import.expiredAnnouncement'); await nextTick(); errorSummary.value?.focus() }
    else if (kind === 'conflict') { state.value = { kind: 'CONFLICT' }; announcement.value = t('admin.import.conflictAnnouncement') }
    else { state.value = { kind: 'PREVIEW', preview: snapshot.preview, token: snapshot.token, message: t('admin.import.importUnavailable') }; announcement.value = t('admin.import.unavailableAnnouncement') }
  }
}
function reset() { generation += 1; file.value = undefined; fileField.value?.clear(); state.value = { kind: 'SELECT' }; announcement.value = t('admin.import.resetAnnouncement') }
</script>
