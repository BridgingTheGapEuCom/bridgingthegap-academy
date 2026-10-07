<template>
  <section class="authoring-section authoring-overview" aria-labelledby="authoring-overview-title">
    <AuthoringPageTitle id="authoring-overview-title" :title="t('authoring.overview.title')" :description="t('authoring.overview.description')" />
    <form class="authoring-overview__form" :aria-busy="saving || undefined" novalidate @submit.prevent="save">
      <p v-if="formError" class="authoring-metadata-form__error" role="alert">{{ formError }}</p>
      <div v-if="conflict" class="authoring-metadata-form__conflict" role="status">
        <p>{{ t('authoring.overview.changedElsewhere') }}</p>
        <BtgButton variant="secondary" :disabled="reloading" @click="reloadLatest">{{ reloading ? t('authoring.overview.reloading') : t('authoring.overview.reloadLatest') }}</BtgButton>
      </div>

      <AuthoringSection heading-id="overview-course-basics-title" number="01" :title="t('authoring.overview.basics.title')" :description="t('authoring.overview.basics.description')">
          <BtgFormField :label="t('authoring.overview.basics.titleLabel')" required :error="errors.title" v-slot="{ controlId, describedBy, invalid }">
            <BtgTextInput :id="controlId" v-model="form.title" :disabled="saving || reloading" :aria-describedby="describedBy" :invalid="invalid" required />
          </BtgFormField>
          <div class="authoring-overview__two-column">
            <BtgFormField :label="t('authoring.overview.basics.version')" required :description="t('authoring.overview.basics.versionHelp')" :error="errors.intendedVersion" v-slot="{ controlId, describedBy, invalid }">
              <BtgTextInput :id="controlId" v-model="form.intendedVersion" :disabled="saving || reloading" :aria-describedby="describedBy" :invalid="invalid" required />
            </BtgFormField>
            <BtgFormField :label="t('authoring.overview.basics.language')" required :description="t('authoring.overview.basics.languageHelp')" :error="errors.sourceLanguage" v-slot="{ controlId, describedBy, invalid }">
              <LanguagePicker :id="controlId" v-model="form.sourceLanguage" :labels="languagePickerLabels" :disabled="saving || reloading" :described-by="describedBy" :invalid="invalid" />
            </BtgFormField>
          </div>
      </AuthoringSection>

      <AuthoringSection heading-id="overview-learning-title" number="02" :title="t('authoring.overview.learning.title')" :description="t('authoring.overview.learning.description')">
          <BtgFormField :label="t('authoring.overview.learning.descriptionLabel')" required :description="t('authoring.overview.learning.descriptionHelp')" :error="errors.description" v-slot="{ controlId, describedBy, invalid }">
            <BtgTextarea class="authoring-overview__description" :id="controlId" v-model="form.description" :disabled="saving || reloading" :aria-describedby="describedBy" :invalid="invalid" required />
          </BtgFormField>
          <RepeatableObjectivesEditor v-model="form.objectives" :disabled="saving || reloading" :error="errors.objectives" :label="t('authoring.overview.learning.objectives')" :description="t('authoring.overview.learning.objectivesHelp')" :item-label="t('authoring.overview.learning.objective')" :placeholder="t('authoring.overview.learning.objectivePlaceholder')" :add-label="t('authoring.overview.learning.addObjective')" :reorder-label="(position) => t('authoring.overview.learning.reorderObjective', { position })" :remove-label-for="(position) => t('authoring.overview.learning.removeObjective', { position })" :remove-text="t('authoring.overview.learning.remove')" :added-announcement="(position) => t('authoring.overview.learning.addedObjective', { position })" :removed-announcement="() => t('authoring.overview.learning.removedObjective')" :moved-announcement="(position, total) => t('authoring.overview.learning.movedObjective', { position, total })" :reordering-announcement="(position) => t('authoring.overview.learning.reorderingObjective', { position })" :finished-reordering-announcement="() => t('authoring.overview.learning.finishedReorderingObjectives')" />
      </AuthoringSection>

      <AuthoringSection heading-id="overview-release-notes-title" number="03" :title="t('authoring.overview.releaseNotes.title')" :description="t('authoring.overview.releaseNotes.description')">
          <BtgFormField :label="t('authoring.overview.releaseNotes.changelog')" required :description="t('authoring.overview.releaseNotes.changelogHelp')" :error="errors.changelog" v-slot="{ controlId, describedBy, invalid }">
            <BtgTextarea class="authoring-overview__changelog" :id="controlId" v-model="form.changelog" :disabled="saving || reloading" :aria-describedby="describedBy" :invalid="invalid" required />
          </BtgFormField>
      </AuthoringSection>

      <AuthoringSection heading-id="overview-license-title" number="04" :title="t('authoring.overview.licensing.title')" :description="t('authoring.overview.licensing.description')">
          <div class="authoring-overview__two-column">
            <BtgFormField :label="t('authoring.overview.licensing.kind')" required :description="t('authoring.overview.licensing.kindHelp')" :error="errors.license" v-slot="{ controlId, describedBy, invalid }">
              <BtgSelect :id="controlId" v-model="form.license.kind" :disabled="saving || reloading" :aria-describedby="describedBy" :invalid="invalid" required><option value="STANDARD">{{ t('authoring.overview.licensing.standard') }}</option><option value="ALL_RIGHTS_RESERVED">{{ t('authoring.overview.licensing.allRightsReserved') }}</option><option value="CUSTOM">{{ t('authoring.overview.licensing.custom') }}</option></BtgSelect>
            </BtgFormField>
            <BtgFormField :label="t('authoring.overview.licensing.displayName')" required :description="t('authoring.overview.licensing.displayNameHelp')" :error="errors.license" v-slot="{ controlId, describedBy, invalid }">
              <BtgTextInput :id="controlId" v-model="form.license.display_name" :disabled="saving || reloading" :aria-describedby="describedBy" :invalid="invalid" required />
            </BtgFormField>
          </div>
          <div class="authoring-overview__two-column">
            <BtgFormField :label="t('authoring.overview.licensing.identifier')" :description="t('authoring.overview.licensing.identifierHelp')" v-slot="{ controlId, describedBy }">
              <BtgTextInput :id="controlId" v-model="form.license.identifier" :disabled="saving || reloading" :aria-describedby="describedBy" />
            </BtgFormField>
            <BtgFormField :label="t('authoring.overview.licensing.url')" :description="t('authoring.overview.licensing.urlHelp')" v-slot="{ controlId, describedBy }">
              <BtgTextInput :id="controlId" v-model="form.license.url" :disabled="saving || reloading" :aria-describedby="describedBy" type="url" inputmode="url" />
            </BtgFormField>
          </div>
          <BtgFormField :label="t('authoring.overview.licensing.customText')" :description="t('authoring.overview.licensing.customTextHelp')" v-slot="{ controlId, describedBy }">
            <BtgTextarea class="authoring-overview__license-text" :id="controlId" v-model="form.license.custom_text" :disabled="saving || reloading" :aria-describedby="describedBy" />
          </BtgFormField>
      </AuthoringSection>

      <AuthoringDirtyActionBar :show="dirty" :busy="saving" :disabled="saving || reloading" :save-disabled="conflict" :message="t('authoring.overview.dirty')" :discard-label="t('authoring.overview.discard')" :save-label="t('authoring.overview.save')" :busy-label="t('authoring.overview.saving')" @discard="discardChanges" />
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { APIProblemError } from '../api/client'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { getAuthoringDraft, updateAuthoringDraft, type AuthoringContentLicense, type AuthoringDraft, type AuthoringDraftMetadataPatch } from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgTextInput from '../components/BtgTextInput.vue'
import BtgTextarea from '../components/BtgTextarea.vue'
import BtgSelect from '../components/BtgSelect.vue'
import LanguagePicker from '../components/LanguagePicker.vue'
import RepeatableObjectivesEditor from '../components/RepeatableObjectivesEditor.vue'
import AuthoringDirtyActionBar from '../components/AuthoringDirtyActionBar.vue'
import AuthoringPageTitle from '../components/AuthoringPageTitle.vue'
import AuthoringSection from '../components/AuthoringSection.vue'

type Form = { intendedVersion: string; sourceLanguage: string; title: string; description: string; objectives: string[]; changelog: string; license: AuthoringContentLicense }
const { draft, replaceDraft, markDraftUnavailable } = useAuthoringDraftContext()
const { t } = useI18n()
const original = ref(toForm(draft.value))
const form = reactive(toForm(draft.value))
const errors = reactive<Record<string, string | undefined>>({})
const formError = ref<string>()
const saving = ref(false)
const reloading = ref(false)
const conflict = ref(false)
const dirty = computed(() => JSON.stringify(normalized(form)) !== JSON.stringify(normalized(original.value)))
const languagePickerLabels = computed(() => ({ placeholder: t('authoring.languagePicker.placeholder'), close: t('authoring.languagePicker.close'), open: t('authoring.languagePicker.open'), options: t('authoring.languagePicker.options'), empty: t('authoring.languagePicker.empty'), useTag: (tag: string) => t('authoring.languagePicker.useTag', { tag }), available: (count: number) => t('authoring.languagePicker.available', { count }), selected: (language: string) => t('authoring.languagePicker.selected', { language }) }))
const captureScope = useAuthoringAsyncScope(() => draft.value.id)

function toForm(value: AuthoringDraft): Form { return { intendedVersion: value.intended_version, sourceLanguage: value.source_language, title: value.title, description: value.description, objectives: [...value.objectives], changelog: value.changelog, license: { ...value.license } } }
function objectives(value: string[]) { return value.map((objective) => objective.trim()).filter(Boolean) }
function normalized(value: Form) { return { ...value, objectives: objectives(value.objectives), license: { ...value.license } } }
function clearErrors() { for (const key of Object.keys(errors)) delete errors[key]; formError.value = undefined }
function validate(): boolean {
  clearErrors()
  if (!form.title.trim()) errors.title = t('authoring.overview.validation.title')
  if (!/^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$/.test(form.intendedVersion)) errors.intendedVersion = t('authoring.overview.validation.version')
  if (!form.sourceLanguage.trim()) errors.sourceLanguage = t('authoring.overview.validation.sourceLanguage')
  if (!form.description.trim()) errors.description = t('authoring.overview.validation.description')
  if (!objectives(form.objectives).length) errors.objectives = t('authoring.overview.validation.objectives')
  if (!form.changelog.trim()) errors.changelog = t('authoring.overview.validation.changelog')
  if (!form.license.display_name.trim()) errors.license = t('authoring.overview.validation.license')
  return Object.keys(errors).length === 0
}
function patch(): AuthoringDraftMetadataPatch {
  const before = normalized(original.value); const after = normalized(form); const result: AuthoringDraftMetadataPatch = { expectedRevision: draft.value.revision }
  if (after.intendedVersion !== before.intendedVersion) result.intendedVersion = after.intendedVersion
  if (after.sourceLanguage !== before.sourceLanguage) result.sourceLanguage = after.sourceLanguage
  if (after.title !== before.title) result.title = after.title
  if (after.description !== before.description) result.description = after.description
  if (JSON.stringify(after.objectives) !== JSON.stringify(before.objectives)) result.objectives = after.objectives
  if (after.changelog !== before.changelog) result.changelog = after.changelog
  if (JSON.stringify(after.license) !== JSON.stringify(before.license)) result.license = after.license
  return result
}
function discardChanges() {
  if (saving.value || reloading.value || conflict.value) return
  clearErrors()
  Object.assign(form, toForm(draft.value))
}
async function save() {
  const isCurrent = captureScope()
  if (saving.value || reloading.value || conflict.value || !dirty.value || !validate()) return
  saving.value = true
  try {
    const updated = await updateAuthoringDraft(draft.value.id, patch())
    if (!isCurrent()) return
    original.value = toForm(updated); Object.assign(form, toForm(updated)); replaceDraft(updated)
  } catch (error) {
    if (!isCurrent()) return
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
    if (error instanceof APIProblemError && error.status === 409) { conflict.value = true; return }
    formError.value = error instanceof APIProblemError && error.status === 400 ? t('authoring.overview.saveInvalid') : t('authoring.overview.saveUnavailable')
  } finally { if (isCurrent()) saving.value = false }
}
async function reloadLatest() {
  const isCurrent = captureScope()
  if (reloading.value || saving.value) return
  reloading.value = true; formError.value = undefined
  try {
    const latest = await getAuthoringDraft(draft.value.id)
    if (!isCurrent()) return
    original.value = toForm(latest); Object.assign(form, toForm(latest)); replaceDraft(latest); conflict.value = false
  } catch (error) {
    if (!isCurrent()) return
    if (error instanceof APIProblemError && error.status === 404) markDraftUnavailable()
    else formError.value = t('authoring.overview.reloadUnavailable')
  } finally { if (isCurrent()) reloading.value = false }
}
</script>
