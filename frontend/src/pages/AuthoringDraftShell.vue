<template>
  <BtgPageContainer as="section" class="authoring-shell" :aria-labelledby="isLessonWorkspace ? undefined : 'authoring-draft-title'">
    <div v-if="auth.state.value.status === 'bootstrapping'" class="authoring-shell__state" role="status">
      <h1 id="authoring-draft-title">Checking your session</h1>
      <p>One moment while we check whether you are signed in.</p>
    </div>

    <div v-else-if="auth.state.value.status === 'unavailable'" class="authoring-shell__state">
      <h1 id="authoring-draft-title">Authoring unavailable</h1>
      <p>We couldn’t check your session right now. Please try again.</p>
      <BtgButton variant="secondary" @click="retryBootstrap">Try again</BtgButton>
    </div>

    <div v-else-if="auth.state.value.status === 'unauthenticated'" class="authoring-shell__state" role="status">
      <h1 id="authoring-draft-title">Sign in required</h1>
      <p>Taking you to sign in.</p>
    </div>

    <div v-else-if="state.kind === 'loading'" class="authoring-shell__state" role="status">
      <h1 id="authoring-draft-title">Loading draft…</h1>
      <p>One moment while we load this Authoring workspace.</p>
    </div>

    <div v-else-if="state.kind === 'not-found'" class="authoring-shell__state">
      <h1 id="authoring-draft-title">Draft unavailable</h1>
      <p>This Draft is not available.</p>
      <RouterLink to="/">Return home</RouterLink>
    </div>

    <div v-else-if="state.kind === 'unavailable'" class="authoring-shell__state">
      <h1 id="authoring-draft-title">Authoring unavailable</h1>
      <p>We couldn’t load this Draft right now. Please try again.</p>
      <BtgButton variant="secondary" @click="load">Try again</BtgButton>
    </div>

    <div v-else-if="isLessonPreview" class="authoring-shell__content authoring-shell__content--preview">
      <RouterView />
    </div>

    <div v-else class="authoring-shell__content">
      <template v-if="!isLessonWorkspace">
        <AuthoringDraftHeader :title="state.draft.title" :metadata="draftMetadata" :tabs="draftTabs" />
      </template>

      <div class="authoring-shell__section">
        <RouterView />
      </div>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import { useAuth } from '../auth/auth'
import { loginLocation, routeReturnPath } from '../auth/navigation'
import { authoringDraftPath, getAuthoringDraft, InvalidAuthoringDraftIDError, type AuthoringDraft } from '../authoring/authoring'
import { authoringDraftContextKey } from '../authoring/draftContext'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import AuthoringDraftHeader from '../components/AuthoringDraftHeader.vue'
import { languageLabel } from '../i18n/languages'

type State = { kind: 'loading' } | { kind: 'ready'; draft: AuthoringDraft } | { kind: 'not-found' } | { kind: 'unavailable' }

const auth = useAuth()
const route = useRoute()
const router = useRouter()
const state = ref<State>({ kind: 'loading' })
const draft = ref<AuthoringDraft>()
const isLessonPreview = computed(() => route.name === 'authoring-draft-lesson-preview')
const isLessonWorkspace = computed(() => route.name === 'authoring-draft-lesson' || route.name === 'authoring-draft-lesson-content' || route.name === 'authoring-draft-lesson-prerequisites')
const draftMetadata = computed(() => state.value.kind === 'ready' ? [
  { label: 'Intended version', value: state.value.draft.intended_version },
  { label: 'Status', value: draftStatus(state.value.draft.status) },
  { label: 'Source language', value: languageLabel(state.value.draft.source_language) },
  { label: 'Last updated', value: updatedAt(state.value.draft.updated_at) },
] : [])
const draftTabs = computed(() => state.value.kind === 'ready' ? [
  { label: 'Overview', to: authoringDraftPath(state.value.draft.id, 'overview') },
  { label: 'Structure', to: authoringDraftPath(state.value.draft.id, 'structure') },
  { label: 'Members', to: authoringDraftPath(state.value.draft.id, 'members') },
  { label: 'Assessments', to: `/authoring/drafts/${encodeURIComponent(state.value.draft.id)}/assessments` },
  { label: 'Review', to: authoringDraftPath(state.value.draft.id, 'review') },
] : [])

let requestVersion = 0
let active = true

provide(authoringDraftContextKey, {
  draft,
  replaceDraft: (nextDraft) => {
    if (!active || !draft.value || nextDraft.id !== draft.value.id || nextDraft.revision < draft.value.revision) return
    draft.value = nextDraft
    state.value = { kind: 'ready', draft: nextDraft }
  },
  markDraftUnavailable: () => {
    requestVersion += 1
    state.value = { kind: 'not-found' }
    void nextTick(() => { draft.value = undefined })
  },
})

watch(
  [() => auth.state.value, () => route.params.draftId],
  ([session]) => {
    const status = session.status
    if (status === 'authenticated') {
      void load()
      return
    }
    requestVersion += 1
    state.value = { kind: 'loading' }
    void nextTick(() => { draft.value = undefined })
    if (status === 'unauthenticated') void router.replace(loginLocation(routeReturnPath(route)))
  },
  { immediate: true, flush: 'sync' },
)

onBeforeUnmount(() => { active = false })

async function load() {
  const generation = ++requestVersion
  if (auth.state.value.status !== 'authenticated') return
  const draftID = typeof route.params.draftId === 'string' ? route.params.draftId : ''
  state.value = { kind: 'loading' }
  await nextTick()
  if (!active || generation !== requestVersion) return
  draft.value = undefined
  try {
    const loadedDraft = await getAuthoringDraft(draftID)
    if (!active || generation !== requestVersion || auth.state.value.status !== 'authenticated') return
    draft.value = loadedDraft
    state.value = { kind: 'ready', draft: loadedDraft }
  } catch (error) {
    if (!active || generation !== requestVersion) return
    draft.value = undefined
    state.value = error instanceof InvalidAuthoringDraftIDError || (error instanceof APIProblemError && error.status === 404)
      ? { kind: 'not-found' }
      : { kind: 'unavailable' }
  }
}

async function retryBootstrap() {
  await auth.bootstrapSession()
}

function draftStatus(status: AuthoringDraft['status']): string {
  return status === 'ACTIVE' ? 'Active Draft' : 'Abandoned Draft'
}
function updatedAt(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}
</script>
