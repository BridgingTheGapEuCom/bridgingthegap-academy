<template>
  <BtgPageContainer as="section" class="authoring-create" aria-labelledby="authoring-create-title">
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
      <header class="authoring-create__heading">
        <p class="authoring-create__eyebrow">Bridging the Gap Academy</p>
        <h1 id="authoring-create-title">Create Draft</h1>
        <p class="authoring-create__intro">Start with the core course metadata. You can add structure and content in the Draft workspace.</p>
      </header>

      <form class="authoring-create__form" :aria-busy="creating || undefined" novalidate @submit.prevent="create">
        <p v-if="formError" class="authoring-metadata-form__error" role="alert">{{ formError }}</p>
        <p v-if="creating" class="authoring-create__pending" role="status">Creating Draft…</p>

        <section class="authoring-create__panel" aria-labelledby="course-basics-title">
          <header class="authoring-create__panel-header">
            <h2 id="course-basics-title"><span aria-hidden="true">1.</span> Course basics</h2>
            <p>Set the essential details for your course.</p>
          </header>
          <div class="authoring-create__panel-body">
            <BtgFormField label="Title" required :error="errors.title" v-slot="{ controlId, describedBy, invalid }">
              <BtgTextInput :id="controlId" v-model="form.title" :disabled="creating" :aria-describedby="describedBy" :invalid="invalid" placeholder="Enter a course title" required />
            </BtgFormField>
            <div class="authoring-create__basics-grid">
              <BtgFormField label="Intended version" required description="Use a three-part version such as 0.1.0." :error="errors.intendedVersion" v-slot="{ controlId, describedBy, invalid }">
                <BtgTextInput :id="controlId" v-model="form.intendedVersion" :disabled="creating" :aria-describedby="describedBy" :invalid="invalid" required />
              </BtgFormField>
              <BtgFormField label="Source language" required description="Language used by the original Course content." :error="errors.sourceLanguage" v-slot="{ controlId, describedBy, invalid }">
                <LanguagePicker :id="controlId" v-model="form.sourceLanguage" :disabled="creating" :described-by="describedBy" :invalid="invalid" />
              </BtgFormField>
            </div>
          </div>
        </section>

        <section class="authoring-create__panel" aria-labelledby="learning-overview-title">
          <header class="authoring-create__panel-header">
            <h2 id="learning-overview-title"><span aria-hidden="true">2.</span> Learning overview</h2>
            <p>Describe what the course is about and what learners will be able to do.</p>
          </header>
          <div class="authoring-create__panel-body">
            <BtgFormField label="Description" required description="Summarize the purpose, scope, and audience for this course." :error="errors.description" v-slot="{ controlId, describedBy, invalid }">
              <textarea class="authoring-create__description" :id="controlId" v-model="form.description" :disabled="creating" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" placeholder="Enter a brief description of the course." required />
            </BtgFormField>
            <RepeatableObjectivesEditor v-model="form.objectives" :disabled="creating" :error="errors.objectives" description="Add the learning outcomes learners should achieve." />
          </div>
        </section>

        <section class="authoring-create__panel" aria-labelledby="release-notes-title">
          <header class="authoring-create__panel-header">
            <h2 id="release-notes-title"><span aria-hidden="true">3.</span> Release notes</h2>
            <p>Document the changes in this initial version.</p>
          </header>
          <div class="authoring-create__panel-body">
            <BtgFormField label="Changelog" required description="Describe the initial Draft version." :error="errors.changelog" v-slot="{ controlId, describedBy, invalid }">
              <textarea class="authoring-create__changelog" :id="controlId" v-model="form.changelog" :disabled="creating" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" required />
            </BtgFormField>
          </div>
        </section>

        <div class="authoring-create__actions">
          <BtgButton type="button" variant="secondary" :disabled="creating" @click="cancel">Cancel</BtgButton>
          <BtgButton type="submit" :disabled="creating">{{ creating ? 'Creating…' : 'Create Draft' }}</BtgButton>
        </div>
      </form>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import { authoringDraftPath, createAuthoringDraft, type AuthoringDraftCreate } from '../authoring/authoring'
import { useAuth } from '../auth/auth'
import { loginLocation, routeReturnPath } from '../auth/navigation'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgTextInput from '../components/BtgTextInput.vue'
import LanguagePicker from '../components/LanguagePicker.vue'
import RepeatableObjectivesEditor from '../components/RepeatableObjectivesEditor.vue'

type Form = { title: string; intendedVersion: string; sourceLanguage: string; description: string; objectives: string[]; changelog: string }
const auth = useAuth()
const router = useRouter()
const route = useRoute()
const form = reactive<Form>({ title: '', intendedVersion: '0.1.0', sourceLanguage: 'en', description: '', objectives: [''], changelog: 'Initial Draft.' })
const errors = reactive<Record<string, string | undefined>>({})
const formError = ref<string>()
const creating = ref(false)
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
function clearErrors() { for (const key of Object.keys(errors)) delete errors[key]; formError.value = undefined }
function validate(): boolean {
  clearErrors()
  if (!form.title.trim()) errors.title = 'Enter a title.'
  if (!/^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$/.test(form.intendedVersion)) errors.intendedVersion = 'Enter a version such as 0.1.0.'
  if (!form.sourceLanguage.trim()) errors.sourceLanguage = 'Enter or select a source language.'
  if (!form.description.trim()) errors.description = 'Enter a description.'
  if (!objectives().length) errors.objectives = 'Enter at least one learning objective.'
  if (!form.changelog.trim()) errors.changelog = 'Enter a changelog.'
  return Object.keys(errors).length === 0
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
