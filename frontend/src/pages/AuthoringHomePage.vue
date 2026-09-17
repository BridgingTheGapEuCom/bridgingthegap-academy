<template>
  <BtgPageContainer as="section" class="authoring-home" width="reading" aria-labelledby="authoring-home-title">
    <div v-if="auth.state.value.status === 'bootstrapping'" class="authoring-home__state" role="status">
      <h1 id="authoring-home-title">Checking your session</h1>
      <p>One moment while we check whether you are signed in.</p>
    </div>
    <div v-else-if="auth.state.value.status === 'unavailable'" class="authoring-home__state" role="alert">
      <h1 id="authoring-home-title">Authoring unavailable</h1>
      <p>We couldn’t check your session right now. Please try again.</p>
      <BtgButton variant="secondary" @click="retryBootstrap">Try again</BtgButton>
    </div>
    <div v-else-if="auth.state.value.status === 'unauthenticated'" class="authoring-home__state" role="status">
      <h1 id="authoring-home-title">Sign in required</h1>
      <p>Taking you to sign in.</p>
    </div>
    <div v-else-if="state.kind === 'loading'" class="authoring-home__state" role="status">
      <h1 id="authoring-home-title">Authoring</h1>
      <p>Loading your Drafts…</p>
    </div>
    <div v-else-if="state.kind === 'unavailable'" class="authoring-home__state" role="alert">
      <h1 id="authoring-home-title">Authoring unavailable</h1>
      <p>We couldn’t load your Drafts right now. Please try again.</p>
      <BtgButton variant="secondary" @click="load">Try again</BtgButton>
    </div>
    <div v-else class="authoring-home__content">
      <p class="authoring-home__eyebrow">Bridging the Gap Academy</p>
      <h1 id="authoring-home-title">Authoring</h1>
      <p class="authoring-home__intro">Maintain course Drafts before publication.</p>
      <p><BtgButton @click="createDraft">Create Draft</BtgButton></p>
      <section aria-labelledby="authoring-drafts-title">
        <h2 id="authoring-drafts-title">Your Drafts</h2>
        <p v-if="state.drafts.length === 0" class="authoring-home__empty" role="status">You do not currently have any course Drafts.</p>
        <ol v-else class="authoring-home__list">
          <li v-for="draft in state.drafts" :key="draft.id" class="authoring-home__item">
            <article>
              <h3>{{ draft.title }}</h3>
              <dl class="authoring-home__metadata">
                <div><dt>Intended version</dt><dd>{{ draft.intendedVersion }}</dd></div>
                <div><dt>Status</dt><dd>{{ statusLabel(draft.status) }}</dd></div>
              </dl>
              <RouterLink class="authoring-home__open" :to="authoringDraftPath(draft.id)">Open Draft: {{ draft.title }}</RouterLink>
            </article>
          </li>
        </ol>
      </section>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../auth/auth'
import { authoringDraftPath, listAuthoringDrafts, type AuthoringDraftSummary } from '../authoring/authoring'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'

type State = { kind: 'loading' } | { kind: 'ready'; drafts: AuthoringDraftSummary[] } | { kind: 'unavailable' }
const auth = useAuth()
const router = useRouter()
const state = ref<State>({ kind: 'loading' })
let requestVersion = 0
let active = true

watch(
  () => auth.state.value,
  (session) => {
    const status = session.status
    if (status === 'authenticated') {
      void load()
      return
    }
    requestVersion += 1
    state.value = { kind: 'loading' }
    if (status === 'unauthenticated') void router.replace('/login')
  },
  { immediate: true, flush: 'sync' },
)
onBeforeUnmount(() => { active = false })

async function load() {
  const generation = ++requestVersion
  if (auth.state.value.status !== 'authenticated') return
  state.value = { kind: 'loading' }
  try {
    const response = await listAuthoringDrafts()
    if (!active || generation !== requestVersion || auth.state.value.status !== 'authenticated') return
    state.value = { kind: 'ready', drafts: response.drafts }
  } catch {
    if (!active || generation !== requestVersion) return
    state.value = { kind: 'unavailable' }
  }
}

async function retryBootstrap() { await auth.bootstrapSession() }
function statusLabel(status: AuthoringDraftSummary['status']): string { return status === 'ACTIVE' ? 'Active Draft' : 'Abandoned Draft' }
function createDraft() { void router.push('/authoring/new') }
</script>
