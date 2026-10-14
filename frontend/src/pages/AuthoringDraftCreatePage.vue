<template>
  <BtgPageContainer as="section" class="authoring-create" aria-labelledby="authoring-create-title">
    <RouterLink class="btg-button btg-button--tertiary authoring-create__back" to="/authoring">
      <BtgIcon name="arrowLeft" decorative />{{ t('authoring.create.back') }}
    </RouterLink>

    <BtgPanel v-if="auth.state.value.status === 'bootstrapping'" class="authoring-create__state" variant="subtle" role="status">
      <BtgPageTitle id="authoring-create-title" :title="t('authoring.create.checkingSession')" :description="t('authoring.create.checkingSessionDescription')" />
    </BtgPanel>
    <BtgPanel v-else-if="auth.state.value.status === 'unavailable'" class="authoring-create__state" variant="subtle" role="alert">
      <BtgPageTitle id="authoring-create-title" :title="t('authoring.create.unavailable')" :description="t('authoring.create.sessionUnavailable')" />
      <BtgButton variant="secondary" @click="retryBootstrap">{{ t('authoring.entry.tryAgain') }}</BtgButton>
    </BtgPanel>
    <BtgPanel v-else-if="auth.state.value.status === 'unauthenticated'" class="authoring-create__state" variant="subtle" role="status">
      <BtgPageTitle id="authoring-create-title" :title="t('authoring.create.signInRequired')" :description="t('authoring.create.redirecting')" />
    </BtgPanel>
    <div v-else class="authoring-create__content">
      <BtgPageTitle id="authoring-create-title" :title="t('authoring.create.title')" :description="t('authoring.create.description')" />

      <form class="authoring-create__form" :aria-busy="creating || undefined" novalidate @submit.prevent="create">
        <p v-if="formError" class="authoring-metadata-form__error" role="alert">{{ formError }}</p>
        <p v-if="creating" class="authoring-create__pending" role="status">{{ t('authoring.create.submitting') }}</p>

        <BtgPanel class="authoring-create__panel">
          <BtgSection heading-id="course-basics-title">
            <template #heading>{{ t('authoring.create.basics.title') }}</template>
            <p class="authoring-create__section-description">{{ t('authoring.create.basics.description') }}</p>
            <div class="authoring-create__section-fields">
              <BtgFormField control-id="authoring-create-title-field" :label="t('authoring.create.basics.titleLabel')" required :required-label="t('authoring.create.required')" :error="fieldError('title')" v-slot="{ controlId, describedBy, invalid }">
                <BtgTextInput :id="controlId" v-model="form.title" :disabled="creating" :aria-describedby="describedBy" :invalid="invalid" :placeholder="t('authoring.create.basics.titlePlaceholder')" required />
              </BtgFormField>
              <div class="authoring-create__basics-grid">
                <BtgFormField control-id="authoring-create-version" :label="t('authoring.create.basics.version')" required :required-label="t('authoring.create.required')" :description="t('authoring.create.basics.versionHelp')" :error="fieldError('intendedVersion')" v-slot="{ controlId, describedBy, invalid }">
                  <BtgTextInput :id="controlId" v-model="form.intendedVersion" :disabled="creating" :aria-describedby="describedBy" :invalid="invalid" required />
                </BtgFormField>
                <BtgFormField control-id="authoring-create-language" :label="t('authoring.create.basics.sourceLanguage')" required :required-label="t('authoring.create.required')" :description="t('authoring.create.basics.sourceLanguageHelp')" :error="fieldError('sourceLanguage')" v-slot="{ controlId, describedBy, invalid }">
                  <LanguagePicker :id="controlId" v-model="form.sourceLanguage" :labels="languagePickerLabels" :disabled="creating" :described-by="describedBy" :invalid="invalid" />
                </BtgFormField>
              </div>
            </div>
          </BtgSection>
        </BtgPanel>

        <BtgPanel class="authoring-create__panel">
          <BtgSection heading-id="learning-overview-title">
            <template #heading>{{ t('authoring.create.learning.title') }}</template>
            <p class="authoring-create__section-description">{{ t('authoring.create.learning.description') }}</p>
            <div class="authoring-create__section-fields">
              <BtgFormField control-id="authoring-create-description" :label="t('authoring.create.learning.descriptionLabel')" required :required-label="t('authoring.create.required')" :description="t('authoring.create.learning.descriptionHelp')" :error="fieldError('description')" v-slot="{ controlId, describedBy, invalid }">
                <BtgTextarea class="authoring-create__description" :id="controlId" v-model="form.description" :disabled="creating" :aria-describedby="describedBy" :invalid="invalid" :placeholder="t('authoring.create.learning.descriptionPlaceholder')" required />
              </BtgFormField>
              <RepeatableObjectivesEditor v-model="form.objectives" :disabled="creating" :error="fieldError('objectives')" :required-label="t('authoring.create.required')" :label="t('authoring.create.learning.objectives')" :description="t('authoring.create.learning.objectivesHelp')" :item-label="t('authoring.create.learning.objective')" :placeholder="t('authoring.create.learning.objectivePlaceholder')" :add-label="t('authoring.create.learning.addObjective')" :reorder-label="(position) => t('authoring.create.learning.reorderObjective', { position })" :remove-label-for="(position) => t('authoring.create.learning.removeObjective', { position })" :remove-text="t('authoring.create.learning.remove')" :added-announcement="(position) => t('authoring.create.learning.addedObjective', { position })" :removed-announcement="() => t('authoring.create.learning.removedObjective')" :moved-announcement="(position, total) => t('authoring.create.learning.movedObjective', { position, total })" :reordering-announcement="(position) => t('authoring.create.learning.reorderingObjective', { position })" :finished-reordering-announcement="() => t('authoring.create.learning.finishedReorderingObjectives')" />
            </div>
          </BtgSection>
        </BtgPanel>

        <BtgPanel class="authoring-create__panel">
          <BtgSection heading-id="release-notes-title">
            <template #heading>{{ t('authoring.create.releaseNotes.title') }}</template>
            <p class="authoring-create__section-description">{{ t('authoring.create.releaseNotes.description') }}</p>
            <div class="authoring-create__section-fields">
              <BtgFormField control-id="authoring-create-changelog" :label="t('authoring.create.releaseNotes.changelog')" required :required-label="t('authoring.create.required')" :description="t('authoring.create.releaseNotes.changelogHelp')" :error="fieldError('changelog')" v-slot="{ controlId, describedBy, invalid }">
                <BtgTextarea class="authoring-create__changelog" :id="controlId" v-model="form.changelog" :disabled="creating" :aria-describedby="describedBy" :invalid="invalid" required />
              </BtgFormField>
            </div>
          </BtgSection>
        </BtgPanel>

        <div class="authoring-create__actions">
          <BtgButton type="button" variant="secondary" :disabled="creating" @click="cancel">{{ t('authoring.create.cancel') }}</BtgButton>
          <BtgButton type="submit" leading-icon="add" :loading="creating">{{ creating ? t('authoring.create.submitting') : t('authoring.create.submit') }}</BtgButton>
        </div>
      </form>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import { authoringDraftPath, createAuthoringDraft, type AuthoringDraftCreate } from '../authoring/authoring'
import { useAuth } from '../auth/auth'
import { loginLocation, routeReturnPath } from '../auth/navigation'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgIcon from '../components/BtgIcon.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import BtgSection from '../components/BtgSection.vue'
import BtgTextInput from '../components/BtgTextInput.vue'
import BtgTextarea from '../components/BtgTextarea.vue'
import LanguagePicker from '../components/LanguagePicker.vue'
import RepeatableObjectivesEditor from '../components/RepeatableObjectivesEditor.vue'

type Form = { title: string; intendedVersion: string; sourceLanguage: string; description: string; objectives: string[]; changelog: string }
type Field = 'title' | 'intendedVersion' | 'sourceLanguage' | 'description' | 'objectives' | 'changelog'
type ValidationCode = 'title_required' | 'version_invalid' | 'source_language_required' | 'description_required' | 'objectives_required' | 'changelog_required'

const auth = useAuth()
const router = useRouter()
const route = useRoute()
const { t } = useI18n()
// Course language remains a course-domain default and never follows the application locale.
const form = reactive<Form>({ title: '', intendedVersion: '0.1.0', sourceLanguage: 'en', description: '', objectives: [''], changelog: t('authoring.create.releaseNotes.initialChangelog') })
const errors = reactive<Partial<Record<Field, ValidationCode>>>({})
const formError = ref<string>()
const creating = ref(false)
const languagePickerLabels = computed(() => ({ placeholder: t('authoring.languagePicker.placeholder'), close: t('authoring.languagePicker.close'), open: t('authoring.languagePicker.open'), options: t('authoring.languagePicker.options'), empty: t('authoring.languagePicker.empty'), useTag: (tag: string) => t('authoring.languagePicker.useTag', { tag }), available: (count: number) => t('authoring.languagePicker.available', { count }), selected: (language: string) => t('authoring.languagePicker.selected', { language }) }))
let active = true
let requestVersion = 0

watch(
  () => auth.state.value.status,
  (status) => {
    requestVersion += 1
    if (status === 'unauthenticated') void router.replace(loginLocation(routeReturnPath(route)))
  },
  { immediate: true, flush: 'sync' },
)
onBeforeUnmount(() => { active = false; requestVersion += 1 })

function objectives() { return form.objectives.map((objective) => objective.trim()).filter(Boolean) }
function fieldError(field: Field) { const code = errors[field]; return code ? t(`authoring.create.validation.${code}`) : undefined }
function clearErrors() { for (const key of Object.keys(errors)) delete errors[key as Field]; formError.value = undefined }
function validate(): boolean {
  clearErrors()
  if (!form.title.trim()) errors.title = 'title_required'
  if (!/^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$/.test(form.intendedVersion)) errors.intendedVersion = 'version_invalid'
  if (!form.sourceLanguage.trim()) errors.sourceLanguage = 'source_language_required'
  if (!form.description.trim()) errors.description = 'description_required'
  if (!objectives().length) errors.objectives = 'objectives_required'
  if (!form.changelog.trim()) errors.changelog = 'changelog_required'
  if (Object.keys(errors).length) { void nextTick(focusFirstError); return false }
  return true
}
function focusFirstError() {
  const targets: Record<Field, string> = {
    title: '#authoring-create-title-field', intendedVersion: '#authoring-create-version', sourceLanguage: '#authoring-create-language', description: '#authoring-create-description', objectives: '.authoring-create .objectives-editor input', changelog: '#authoring-create-changelog',
  }
  const first = (Object.keys(errors) as Field[])[0]
  if (first) document.querySelector<HTMLElement>(targets[first])?.focus()
}
function input(): AuthoringDraftCreate {
  return { title: form.title, intendedVersion: form.intendedVersion, sourceLanguage: form.sourceLanguage, description: form.description, objectives: objectives(), changelog: form.changelog }
}
async function create() {
  if (creating.value || auth.state.value.status !== 'authenticated' || !validate()) return
  const generation = ++requestVersion
  creating.value = true
  try {
    const draft = await createAuthoringDraft(input())
    if (!active || generation !== requestVersion || auth.state.value.status !== 'authenticated') return
    await router.push(authoringDraftPath(draft.id))
  } catch (error) {
    if (!active || generation !== requestVersion) return
    formError.value = error instanceof APIProblemError && error.status === 400 ? t('authoring.create.invalid') : t('authoring.create.unavailableError')
  } finally {
    if (active && generation === requestVersion) creating.value = false
  }
}
async function retryBootstrap() { await auth.bootstrapSession() }
function cancel() { void router.push('/authoring') }
</script>
