<template>
  <section class="authoring-section" aria-labelledby="authoring-overview-title">
    <h2 id="authoring-overview-title">Overview</h2>
    <p class="authoring-section__intro">Update this Draft’s metadata, then save your changes when you are ready.</p>
    <form class="authoring-metadata-form" :aria-busy="saving" novalidate @submit.prevent="save">
      <p v-if="formError" class="authoring-metadata-form__error" role="alert">{{ formError }}</p>
      <div v-if="conflict" class="authoring-metadata-form__conflict" role="status">
        <p>This Draft changed elsewhere. Your edits are still here, but you need to reload the latest Draft before saving again.</p>
        <BtgButton variant="secondary" :disabled="reloading" @click="reloadLatest">{{ reloading ? 'Reloading…' : 'Reload latest Draft' }}</BtgButton>
      </div>

      <BtgFormField label="Title" required :error="errors.title" v-slot="{ controlId, describedBy, invalid }"><BtgTextInput :id="controlId" v-model="form.title" :disabled="saving || reloading" :aria-describedby="describedBy" :invalid="invalid" required /></BtgFormField>
      <BtgFormField label="Intended version" required description="Use a three-part version such as 1.0.0." :error="errors.intendedVersion" v-slot="{ controlId, describedBy, invalid }"><BtgTextInput :id="controlId" v-model="form.intendedVersion" :disabled="saving || reloading" :aria-describedby="describedBy" :invalid="invalid" required /></BtgFormField>
      <BtgFormField label="Source language" required description="Use the language tag already used for this Draft, such as en or en-GB." :error="errors.sourceLanguage" v-slot="{ controlId, describedBy, invalid }"><BtgTextInput :id="controlId" v-model="form.sourceLanguage" :disabled="saving || reloading" :aria-describedby="describedBy" :invalid="invalid" required /></BtgFormField>
      <BtgFormField label="Description" required :error="errors.description" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" v-model="form.description" :disabled="saving || reloading" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" required /></BtgFormField>
      <BtgFormField label="Learning objectives" required description="Enter one objective per line." :error="errors.objectives" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" v-model="form.objectivesText" :disabled="saving || reloading" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" required /></BtgFormField>
      <BtgFormField label="Changelog" required :error="errors.changelog" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" v-model="form.changelog" :disabled="saving || reloading" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" required /></BtgFormField>

      <fieldset class="authoring-metadata-form__license">
        <legend>Course content license</legend>
        <BtgFormField label="License type" required :error="errors.license" v-slot="{ controlId, describedBy, invalid }"><select :id="controlId" v-model="form.license.kind" :disabled="saving || reloading" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" required><option value="STANDARD">Standard license</option><option value="ALL_RIGHTS_RESERVED">All rights reserved</option><option value="CUSTOM">Custom license</option></select></BtgFormField>
        <BtgFormField label="License display name" required v-slot="{ controlId, describedBy }"><BtgTextInput :id="controlId" v-model="form.license.display_name" :disabled="saving || reloading" :aria-describedby="describedBy" required /></BtgFormField>
        <BtgFormField label="License identifier" v-slot="{ controlId, describedBy }"><BtgTextInput :id="controlId" v-model="form.license.identifier" :disabled="saving || reloading" :aria-describedby="describedBy" /></BtgFormField>
        <BtgFormField label="License URL" v-slot="{ controlId, describedBy }"><BtgTextInput :id="controlId" v-model="form.license.url" :disabled="saving || reloading" :aria-describedby="describedBy" type="url" inputmode="url" /></BtgFormField>
        <BtgFormField label="Custom license text" v-slot="{ controlId, describedBy }"><textarea :id="controlId" v-model="form.license.custom_text" :disabled="saving || reloading" :aria-describedby="describedBy" /></BtgFormField>
      </fieldset>
      <p v-if="dirty" class="authoring-metadata-form__unsaved" role="status">You have unsaved changes.</p>
      <div class="authoring-metadata-form__actions"><BtgButton type="submit" :disabled="!dirty || saving || reloading || conflict">{{ saving ? 'Saving…' : 'Save changes' }}</BtgButton></div>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { APIProblemError } from '../api/client'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { getAuthoringDraft, updateAuthoringDraft, type AuthoringContentLicense, type AuthoringDraft, type AuthoringDraftMetadataPatch } from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgTextInput from '../components/BtgTextInput.vue'

type Form = { intendedVersion: string; sourceLanguage: string; title: string; description: string; objectivesText: string; changelog: string; license: AuthoringContentLicense }
const { draft, replaceDraft, markDraftUnavailable } = useAuthoringDraftContext()
const original = ref(toForm(draft.value))
const form = reactive(toForm(draft.value))
const errors = reactive<Record<string, string | undefined>>({})
const formError = ref<string>()
const saving = ref(false)
const reloading = ref(false)
const conflict = ref(false)
const dirty = computed(() => JSON.stringify(normalized(form)) !== JSON.stringify(normalized(original.value)))


const captureScope = useAuthoringAsyncScope(() => draft.value.id)

function toForm(value: AuthoringDraft): Form { return { intendedVersion: value.intended_version, sourceLanguage: value.source_language, title: value.title, description: value.description, objectivesText: value.objectives.join('\n'), changelog: value.changelog, license: { ...value.license } } }
function objectives(value: string): string[] { return value.split('\n').map((objective) => objective.trim()).filter(Boolean) }
function normalized(value: Form) { return { ...value, objectives: objectives(value.objectivesText), license: { ...value.license } } }
function clearErrors() { for (const key of Object.keys(errors)) delete errors[key]; formError.value = undefined }
function validate(): boolean {
  clearErrors()
  if (!form.title.trim()) errors.title = 'Enter a title.'
  if (!/^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$/.test(form.intendedVersion)) errors.intendedVersion = 'Enter a version such as 1.0.0.'
  if (!form.sourceLanguage.trim()) errors.sourceLanguage = 'Enter a source language.'
  if (!form.description.trim()) errors.description = 'Enter a description.'
  if (!objectives(form.objectivesText).length) errors.objectives = 'Enter at least one learning objective.'
  if (!form.changelog.trim()) errors.changelog = 'Enter a changelog.'
  if (!form.license.display_name.trim()) errors.license = 'Enter a license display name.'
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
    formError.value = error instanceof APIProblemError && error.status === 400 ? 'We couldn’t save these changes. Check the fields and try again.' : 'We couldn’t save this Draft right now. Please try again.'
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
    else formError.value = 'We couldn’t reload this Draft right now. Please try again.'
  } finally { if (isCurrent()) reloading.value = false }
}
</script>
