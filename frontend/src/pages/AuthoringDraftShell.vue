<template>
  <BtgPageContainer as="section" class="authoring-shell" width="application" aria-labelledby="authoring-draft-title">
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

    <div v-else class="authoring-shell__content">
      <div class="authoring-shell__header">
        <p class="authoring-shell__eyebrow">Authoring Draft</p>
        <h1 id="authoring-draft-title">{{ state.draft.title }}</h1>
        <dl class="authoring-shell__metadata">
          <div><dt>Intended version</dt><dd>{{ state.draft.intended_version }}</dd></div>
          <div><dt>Status</dt><dd>{{ draftStatus(state.draft.status) }}</dd></div>
        </dl>
      </div>

      <nav class="authoring-shell__navigation" aria-label="Draft sections">
        <ul>
          <li><RouterLink :to="authoringDraftPath(state.draft.id, 'overview')">Overview</RouterLink></li>
          <li><RouterLink :to="authoringDraftPath(state.draft.id, 'structure')">Structure</RouterLink></li>
          <li><RouterLink :to="authoringDraftPath(state.draft.id, 'members')">Members</RouterLink></li>
        </ul>
      </nav>

      <div class="authoring-shell__section">
        <RouterView />
      </div>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { onBeforeUnmount, provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import { useAuth } from '../auth/auth'
import { authoringDraftPath, getAuthoringDraft, InvalidAuthoringDraftIDError, type AuthoringDraft } from '../authoring/authoring'
import { authoringDraftContextKey } from '../authoring/draftContext'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'

type State = { kind: 'loading' } | { kind: 'ready'; draft: AuthoringDraft } | { kind: 'not-found' } | { kind: 'unavailable' }

const auth = useAuth()
const route = useRoute()
const router = useRouter()
const state = ref<State>({ kind: 'loading' })
const draft = ref<AuthoringDraft>()

let requestVersion = 0
let active = true

provide(authoringDraftContextKey, {
  draft,
  replaceDraft: (nextDraft) => {
    draft.value = nextDraft
    state.value = { kind: 'ready', draft: nextDraft }
  },
  markDraftUnavailable: () => {
    requestVersion += 1
    draft.value = undefined
    state.value = { kind: 'not-found' }
  },
})

watch(
  () => [auth.state.value.status, route.params.draftId],
  ([status]) => {
    if (status === 'authenticated') {
      void load()
      return
    }
    requestVersion += 1
    draft.value = undefined
    state.value = { kind: 'loading' }
    if (status === 'unauthenticated') void router.replace('/login')
  },
  { immediate: true },
)

onBeforeUnmount(() => { active = false })

async function load() {
  const generation = ++requestVersion
  if (auth.state.value.status !== 'authenticated') return
  const draftID = typeof route.params.draftId === 'string' ? route.params.draftId : ''
  draft.value = undefined
  state.value = { kind: 'loading' }
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
</script>
