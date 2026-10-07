<template>
  <BtgPageContainer as="section" class="authoring-shell" :aria-labelledby="isLessonWorkspace ? undefined : 'authoring-draft-title'">
    <div v-if="auth.state.value.status === 'bootstrapping'" class="authoring-shell__state" role="status">
      <h1 id="authoring-draft-title">{{ t('authoring.draftShell.checkingSession') }}</h1>
      <p>{{ t('authoring.draftShell.checkingSessionDescription') }}</p>
    </div>

    <div v-else-if="auth.state.value.status === 'unavailable'" class="authoring-shell__state">
      <h1 id="authoring-draft-title">{{ t('authoring.draftShell.authoringUnavailable') }}</h1>
      <p>{{ t('authoring.draftShell.sessionUnavailable') }}</p>
      <BtgButton variant="secondary" @click="retryBootstrap">{{ t('common.actions.tryAgain') }}</BtgButton>
    </div>

    <div v-else-if="auth.state.value.status === 'unauthenticated'" class="authoring-shell__state" role="status">
      <h1 id="authoring-draft-title">{{ t('authoring.draftShell.signInRequired') }}</h1>
      <p>{{ t('authoring.draftShell.redirectingToSignIn') }}</p>
    </div>

    <div v-else-if="state.kind === 'loading'" class="authoring-shell__state" role="status">
      <h1 id="authoring-draft-title">{{ t('authoring.draftShell.loading') }}</h1>
      <p>{{ t('authoring.draftShell.loadingDescription') }}</p>
    </div>

    <div v-else-if="state.kind === 'not-found'" class="authoring-shell__state">
      <h1 id="authoring-draft-title">{{ t('authoring.draftShell.draftUnavailable') }}</h1>
      <p>{{ t('authoring.draftShell.notAvailable') }}</p>
      <RouterLink to="/">{{ t('authoring.draftShell.returnHome') }}</RouterLink>
    </div>

    <div v-else-if="state.kind === 'unavailable'" class="authoring-shell__state">
      <h1 id="authoring-draft-title">{{ t('authoring.draftShell.authoringUnavailable') }}</h1>
      <p>{{ t('authoring.draftShell.loadUnavailable') }}</p>
      <BtgButton variant="secondary" @click="load">{{ t('common.actions.tryAgain') }}</BtgButton>
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
import { useI18n } from 'vue-i18n'
import { APIProblemError } from '../api/client'
import { useAuth } from '../auth/auth'
import { loginLocation, routeReturnPath } from '../auth/navigation'
import { authoringDraftPath, getAuthoringDraft, InvalidAuthoringDraftIDError, type AuthoringDraft } from '../authoring/authoring'
import { authoringDraftContextKey } from '../authoring/draftContext'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import AuthoringDraftHeader from '../components/AuthoringDraftHeader.vue'
import { languageLabel } from '../i18n/languages'
import { useApplicationFormatters } from '../i18n/formatters'

type State = { kind: 'loading' } | { kind: 'ready'; draft: AuthoringDraft } | { kind: 'not-found' } | { kind: 'unavailable' }

const auth = useAuth()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const { date } = useApplicationFormatters()
const state = ref<State>({ kind: 'loading' })
const draft = ref<AuthoringDraft>()
const isLessonPreview = computed(() => route.name === 'authoring-draft-lesson-preview')
const isLessonWorkspace = computed(() => route.name === 'authoring-draft-lesson' || route.name === 'authoring-draft-lesson-content' || route.name === 'authoring-draft-lesson-prerequisites')
const draftMetadata = computed(() => state.value.kind === 'ready' ? [
  { label: t('authoring.draftHeader.intendedVersion'), value: state.value.draft.intended_version },
  { label: t('authoring.draftHeader.status'), value: draftStatus(state.value.draft.status) },
  { label: t('authoring.draftHeader.sourceLanguage'), value: languageLabel(state.value.draft.source_language) },
  { label: t('authoring.draftHeader.lastUpdated'), value: date(state.value.draft.updated_at, 'dateTime') },
] : [])
const draftTabs = computed(() => state.value.kind === 'ready' ? [
  { label: t('authoring.draftHeader.tabs.overview'), to: authoringDraftPath(state.value.draft.id, 'overview') },
  { label: t('authoring.draftHeader.tabs.structure'), to: authoringDraftPath(state.value.draft.id, 'structure') },
  { label: t('authoring.draftHeader.tabs.members'), to: authoringDraftPath(state.value.draft.id, 'members') },
  { label: t('authoring.draftHeader.tabs.assessments'), to: `/authoring/drafts/${encodeURIComponent(state.value.draft.id)}/assessments` },
  { label: t('authoring.draftHeader.tabs.review'), to: authoringDraftPath(state.value.draft.id, 'review') },
] : [])

let requestVersion = 0
let active = true

provide(authoringDraftContextKey, {
  draft,
  replaceDraft: (nextDraft) => {
    if (!active || !draft.value || nextDraft.id !== draft.value.id || nextDraft.revision < draft.value.revision) return
    Object.assign(draft.value, nextDraft)
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

function draftStatus(status: AuthoringDraft['status']): string { return status === 'ACTIVE' ? t('authoring.draftHeader.active') : t('authoring.draftHeader.abandoned') }
</script>
