<template>
  <BtgPageContainer as="section" class="authoring-create" width="reading" aria-labelledby="authoring-create-title">
    <RouterLink class="authoring-create__back" to="/authoring">Back to Authoring</RouterLink>
    <div v-if="auth.state.value.status === 'bootstrapping'" class="authoring-create__state" role="status">
      <h1 id="authoring-create-title">Checking your session</h1>
      <p>One moment while we check whether you are signed in.</p>
    </div>
    <div v-else-if="auth.state.value.status === 'unavailable'" class="authoring-create__state" role="alert">
      <h1 id="authoring-create-title">Authoring unavailable</h1>
      <p>We couldn’t check your session right now. Please try again.</p>
      <BtgButton variant="secondary" @click="retryBootstrap">Try again</BtgButton>
    </div>
    <div v-else-if="auth.state.value.status === 'unauthenticated'" class="authoring-create__state" role="status">
      <h1 id="authoring-create-title">Sign in required</h1>
      <p>Taking you to sign in.</p>
    </div>
    <div v-else class="authoring-create__content">
      <p class="authoring-create__eyebrow">Bridging the Gap Academy</p>
      <h1 id="authoring-create-title">Create Draft</h1>
      <p class="authoring-create__intro">Start with the core course metadata. You can add structure and content in the Draft workspace.</p>
      <form class="authoring-metadata-form" :aria-busy="creating || undefined" novalidate @submit.prevent="create">
        <p v-if="formError" class="authoring-metadata-form__error" role="alert">{{ formError }}</p>
        <p v-if="creating" class="authoring-create__pending" role="status">Creating Draft…</p>

        <BtgFormField label="Title" required :error="errors.title" v-slot="{ controlId, describedBy, invalid }"><BtgTextInput :id="controlId" v-model="form.title" :disabled="creating" :aria-describedby="describedBy" :invalid="invalid" required /></BtgFormField>
        <BtgFormField label="Intended version" required description="Use a three-part version such as 0.1.0." :error="errors.intendedVersion" v-slot="{ controlId, describedBy, invalid }"><BtgTextInput :id="controlId" v-model="form.intendedVersion" :disabled="creating" :aria-describedby="describedBy" :invalid="invalid" required /></BtgFormField>
        <BtgFormField label="Source language" required description="Use a language tag such as en or en-GB." :error="errors.sourceLanguage" v-slot="{ controlId, describedBy, invalid }"><BtgTextInput :id="controlId" v-model="form.sourceLanguage" :disabled="creating" :aria-describedby="describedBy" :invalid="invalid" required /></BtgFormField>
        <BtgFormField label="Description" required :error="errors.description" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" v-model="form.description" :disabled="creating" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" required /></BtgFormField>
        <BtgFormField label="Learning objectives" required description="Enter one objective per line." :error="errors.objectives" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" v-model="form.objectivesText" :disabled="creating" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" required /></BtgFormField>
        <BtgFormField label="Changelog" required description="Describe this initial Draft version." :error="errors.changelog" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" v-model="form.changelog" :disabled="creating" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" required /></BtgFormField>

        <div class="authoring-metadata-form__actions">
          <BtgButton type="submit" :disabled="creating">{{ creating ? 'Creating…' : 'Create Draft' }}</BtgButton>
          <BtgButton type="button" variant="secondary" :disabled="creating" @click="cancel">Cancel</BtgButton>
        </div>
      </form>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import { authoringDraftPath, createAuthoringDraft, type AuthoringDraftCreate } from '../authoring/authoring'
import { useAuth } from '../auth/auth'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgTextInput from '../components/BtgTextInput.vue'

type Form = { title: string; intendedVersion: string; sourceLanguage: string; description: string; objectivesText: string; changelog: string }
const auth = useAuth()
const router = useRouter()
const form = reactive<Form>({ title: '', intendedVersion: '0.1.0', sourceLanguage: 'en', description: '', objectivesText: '', changelog: 'Initial Draft.' })
const errors = reactive<Record<string, string | undefined>>({})
const formError = ref<string>()
const creating = ref(false)
let active = true
let requestVersion = 0

watch(
  () => auth.state.value.status,
  (status) => {
    requestVersion += 1
    if (status === 'unauthenticated') void router.replace('/login')
  },
  { immediate: true, flush: 'sync' },
)
onBeforeUnmount(() => { active = false; requestVersion += 1 })

function objectives(value: string): string[] { return value.split('\n').map((objective) => objective.trim()).filter(Boolean) }
function clearErrors() { for (const key of Object.keys(errors)) delete errors[key]; formError.value = undefined }
function validate(): boolean {
  clearErrors()
  if (!form.title.trim()) errors.title = 'Enter a title.'
  if (!/^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$/.test(form.intendedVersion)) errors.intendedVersion = 'Enter a version such as 0.1.0.'
  if (!form.sourceLanguage.trim()) errors.sourceLanguage = 'Enter a source language.'
  if (!form.description.trim()) errors.description = 'Enter a description.'
  if (!objectives(form.objectivesText).length) errors.objectives = 'Enter at least one learning objective.'
  if (!form.changelog.trim()) errors.changelog = 'Enter a changelog.'
  return Object.keys(errors).length === 0
}
function input(): AuthoringDraftCreate {
  return { title: form.title, intendedVersion: form.intendedVersion, sourceLanguage: form.sourceLanguage, description: form.description, objectives: objectives(form.objectivesText), changelog: form.changelog }
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
    formError.value = error instanceof APIProblemError && error.status === 400
      ? 'We couldn’t create this Draft. Check the fields and try again.'
      : 'We couldn’t create this Draft right now. Please try again.'
  } finally {
    if (active && generation === requestVersion) creating.value = false
  }
}
async function retryBootstrap() { await auth.bootstrapSession() }
function cancel() { void router.push('/authoring') }
</script>
