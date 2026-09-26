<template>
  <BtgPageContainer as="section" class="course-import" width="reading" aria-labelledby="course-import-title">
    <p class="sr-only" role="status" aria-live="polite">{{ announcement }}</p>
    <header>
      <p class="course-import__eyebrow">Administration</p>
      <h1 id="course-import-title">Import course package</h1>
      <p>Validate a course package before creating published Course content.</p>
    </header>
    <form v-if="state.kind === 'SELECT' || state.kind === 'VALIDATING' || state.kind === 'ERROR'" @submit.prevent="validate">
      <label for="course-package">Course package</label>
      <input id="course-package" ref="fileInput" type="file" accept=".zip,application/zip" :disabled="state.kind === 'VALIDATING'" @change="selectFile">
      <p v-if="file" class="course-import__selection"><strong>{{ file.name }}</strong> · {{ formatBytes(file.size) }}</p>
      <p v-if="state.kind === 'ERROR'" ref="errorSummary" class="course-import__error" role="alert" tabindex="-1">{{ state.message }}</p>
      <div class="course-import__actions"><BtgButton type="submit" :disabled="!file || state.kind === 'VALIDATING'">{{ state.kind === 'VALIDATING' ? 'Validating package…' : 'Validate package' }}</BtgButton></div>
    </form>
    <section v-else-if="state.kind === 'PREVIEW' || state.kind === 'IMPORTING'" aria-labelledby="course-import-preview-title">
      <h2 id="course-import-preview-title" ref="previewHeading" tabindex="-1">Review course package</h2>
      <div class="course-import__summary">
        <section><h3>Course</h3><dl><div><dt>Title</dt><dd>{{ state.preview.title }}</dd></div><div><dt>Version</dt><dd>{{ state.preview.version }}</dd></div><div><dt>Source language</dt><dd>{{ state.preview.language }}</dd></div><div><dt>License</dt><dd>{{ state.preview.license.displayName }}</dd></div></dl></section>
        <section><h3>Contents</h3><dl><div><dt>Modules</dt><dd>{{ state.preview.moduleCount }}</dd></div><div><dt>Lessons</dt><dd>{{ state.preview.lessonCount }}</dd></div><div><dt>Assessments</dt><dd>{{ state.preview.assessmentCount }}</dd></div><div><dt>Assets</dt><dd>{{ state.preview.assetCount }} · {{ formatBytes(state.preview.assetBytes) }}</dd></div></dl></section>
        <section><h3>Translations</h3><p>{{ state.preview.translationLanguages.length ? state.preview.translationLanguages.join(', ') : 'None' }}</p></section>
        <section v-if="state.preview.attribution.length"><h3>Attribution</h3><ul><li v-for="person in state.preview.attribution" :key="`${person.order}:${person.displayName}`">{{ person.displayName }} ({{ person.role }})</li></ul></section>
      </div>
      <p class="course-import__notice">Import creates published Course content. It does not create local authoring history or assign ownership or editing rights.</p>
      <p v-if="state.kind === 'IMPORTING'" role="status">Importing course package…</p>
      <p v-if="state.kind === 'PREVIEW' && state.message" class="course-import__error" role="alert">{{ state.message }}</p>
      <div class="course-import__actions"><BtgButton :disabled="state.kind === 'IMPORTING'" @click="execute">{{ state.kind === 'IMPORTING' ? 'Importing course…' : 'Import course' }}</BtgButton><BtgButton variant="secondary" :disabled="state.kind === 'IMPORTING'" @click="reset">Choose another package</BtgButton></div>
    </section>
    <section v-else-if="state.kind === 'SUCCESS'" aria-labelledby="course-import-result-title">
      <h2 id="course-import-result-title" ref="resultHeading" tabindex="-1">{{ state.result.status === 'REPLAYED' ? 'Already imported' : 'Course imported' }}</h2>
      <p v-if="state.result.status === 'REPLAYED'">This package was already imported. The existing CourseVersion was reused.</p><p v-else>{{ state.title }} version {{ state.result.semVer }} is now available.</p>
      <p v-if="state.result.translationLanguages.length">Translations: {{ state.result.translationLanguages.join(', ') }}</p>
      <p><RouterLink :to="publishedCourseVersionPath(state.result.courseId, state.result.semVer)">View imported course</RouterLink></p><BtgButton variant="secondary" @click="reset">Import another package</BtgButton>
    </section>
    <section v-else aria-labelledby="course-import-conflict-title"><h2 id="course-import-conflict-title" ref="resultHeading" tabindex="-1">Source/version conflict</h2><p>A different package has already been imported for this source Course version. Import cannot overwrite it.</p><BtgButton variant="secondary" @click="reset">Import another package</BtgButton></section>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref } from 'vue'
import { publishedCourseVersionPath } from '../courses/courses'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import { classifyCoursePackageFailure, executeCoursePackage, formatBytes, packageErrorMessage, previewCoursePackage, type CoursePackageImportResult, type CoursePackagePreviewResponse } from '../portability/portability'

type State = { kind: 'SELECT' } | { kind: 'VALIDATING' } | { kind: 'PREVIEW'; preview: CoursePackagePreviewResponse['preview']; token: string; message?: string } | { kind: 'IMPORTING'; preview: CoursePackagePreviewResponse['preview']; token: string } | { kind: 'SUCCESS'; title: string; result: CoursePackageImportResult } | { kind: 'CONFLICT' } | { kind: 'ERROR'; message: string }
const state = ref<State>({ kind: 'SELECT' }); const file = ref<File>(); const fileInput = ref<HTMLInputElement>(); const previewHeading = ref<HTMLElement>(); const resultHeading = ref<HTMLElement>(); const errorSummary = ref<HTMLElement>(); const announcement = ref(''); let active = true; let generation = 0
onBeforeUnmount(() => { active = false; generation += 1 })
function selectFile(event: Event) { const selected = (event.target as HTMLInputElement).files?.[0]; generation += 1; file.value = selected; state.value = { kind: 'SELECT' }; announcement.value = selected ? `Selected ${selected.name}.` : '' }
async function validate() { if (!file.value || state.value.kind === 'VALIDATING') return; const selected = file.value; const request = ++generation; state.value = { kind: 'VALIDATING' }; announcement.value = 'Validating package.'; try { const response = await previewCoursePackage(selected); if (!active || request !== generation || file.value !== selected) return; state.value = { kind: 'PREVIEW', preview: response.preview, token: response.previewToken }; announcement.value = 'Package validation complete.'; await nextTick(); previewHeading.value?.focus() } catch (error) { if (!active || request !== generation || file.value !== selected) return; state.value = { kind: 'ERROR', message: packageErrorMessage(error) }; announcement.value = 'Package validation failed.'; await nextTick(); errorSummary.value?.focus() } }
async function execute() { if (state.value.kind !== 'PREVIEW') return; const snapshot = state.value; const request = ++generation; state.value = { kind: 'IMPORTING', preview: snapshot.preview, token: snapshot.token }; announcement.value = 'Importing course package.'; try { const result = await executeCoursePackage(snapshot.token); if (!active || request !== generation) return; state.value = { kind: 'SUCCESS', title: snapshot.preview.title, result }; announcement.value = result.status === 'REPLAYED' ? 'Package was already imported.' : 'Course import complete.'; await nextTick(); resultHeading.value?.focus() } catch (error) { if (!active || request !== generation) return; const kind = classifyCoursePackageFailure(error); if (kind === 'expired') { state.value = { kind: 'ERROR', message: 'This package preview expired. Validate the package again before importing.' }; announcement.value = 'Package preview expired.'; await nextTick(); errorSummary.value?.focus() } else if (kind === 'conflict') { state.value = { kind: 'CONFLICT' }; announcement.value = 'Source/version conflict.'; await nextTick(); resultHeading.value?.focus() } else { state.value = { kind: 'PREVIEW', preview: snapshot.preview, token: snapshot.token, message: 'We couldn’t import this package right now. You can try again.' }; announcement.value = 'Course import unavailable.' } } }
function reset() { generation += 1; file.value = undefined; if (fileInput.value) fileInput.value.value = ''; state.value = { kind: 'SELECT' }; announcement.value = 'Ready to validate another package.' }
</script>
