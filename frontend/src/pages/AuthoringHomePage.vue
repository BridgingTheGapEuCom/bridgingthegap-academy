<template>
  <BtgPageContainer as="section" class="authoring-home" aria-labelledby="authoring-home-title">
    <BtgPanel v-if="auth.state.value.status === 'bootstrapping'" class="authoring-home__state" variant="subtle" role="status">
      <BtgPageTitle id="authoring-home-title" :title="t('authoring.entry.checkingSession')" :description="t('authoring.entry.checkingSessionDescription')" />
    </BtgPanel>
    <BtgPanel v-else-if="auth.state.value.status === 'unavailable'" class="authoring-home__state" variant="subtle" role="alert">
      <BtgPageTitle id="authoring-home-title" :title="t('authoring.entry.unavailable')" :description="t('authoring.entry.sessionUnavailable')" />
      <BtgButton variant="secondary" @click="retryBootstrap">{{ t('authoring.entry.tryAgain') }}</BtgButton>
    </BtgPanel>
    <BtgPanel v-else-if="auth.state.value.status === 'unauthenticated'" class="authoring-home__state" variant="subtle" role="status">
      <BtgPageTitle id="authoring-home-title" :title="t('authoring.entry.signInRequired')" :description="t('authoring.entry.redirecting')" />
    </BtgPanel>
    <BtgPanel v-else-if="state.kind === 'loading'" class="authoring-home__state" variant="subtle" role="status">
      <BtgPageTitle id="authoring-home-title" :title="t('authoring.entry.title')" :description="t('authoring.entry.loading')" />
    </BtgPanel>
    <BtgPanel v-else-if="state.kind === 'unavailable'" class="authoring-home__state" variant="subtle" role="alert">
      <BtgPageTitle id="authoring-home-title" :title="t('authoring.entry.unavailable')" :description="t('authoring.entry.loadUnavailable')" />
      <BtgButton variant="secondary" @click="load">{{ t('authoring.entry.tryAgain') }}</BtgButton>
    </BtgPanel>
    <div v-else class="authoring-home__content">
      <header class="authoring-home__header">
        <div>
          <p class="btg-page-eyebrow">{{ t('authoring.entry.eyebrow') }}</p>
          <BtgPageTitle id="authoring-home-title" :title="t('authoring.entry.title')" :description="t('authoring.entry.description')" />
        </div>
        <BtgButton v-if="state.drafts.length" leading-icon="add" @click="createDraft">{{ t('authoring.entry.createDraft') }}</BtgButton>
      </header>

      <BtgSection heading-id="authoring-drafts-title">
        <template #heading>{{ t('authoring.entry.draftsTitle') }}</template>
        <BtgPanel v-if="state.drafts.length === 0" class="authoring-home__empty" variant="subtle" role="status">
          <BtgIcon name="document" size="large" decorative />
          <div>
            <h3>{{ t('authoring.entry.emptyTitle') }}</h3>
            <p>{{ t('authoring.entry.emptyDescription') }}</p>
          </div>
          <BtgButton leading-icon="add" @click="createDraft">{{ t('authoring.entry.createDraft') }}</BtgButton>
        </BtgPanel>
        <ol v-else class="authoring-home__list" :aria-label="t('authoring.entry.draftList')">
          <li v-for="draft in state.drafts" :key="draft.id">
            <BtgPanel as="article" class="authoring-home__item">
              <header class="authoring-home__item-header">
                <h3 :title="draft.title">{{ draft.title }}</h3>
                <BtgBadge :variant="statusVariant(draft.status)">{{ statusLabel(draft.status) }}</BtgBadge>
              </header>
              <dl class="authoring-home__metadata">
                <div><dt>{{ t('authoring.entry.intendedVersion') }}</dt><dd>{{ draft.intendedVersion }}</dd></div>
                <div><dt>{{ t('authoring.entry.lastUpdated') }}</dt><dd>{{ formatters.date(draft.updatedAt, 'dateTime') }}</dd></div>
              </dl>
              <RouterLink class="btg-button btg-button--secondary authoring-home__open" :to="authoringDraftPath(draft.id)">
                <BtgIcon name="edit" decorative />{{ t('authoring.entry.openDraft', { title: draft.title }) }}
              </RouterLink>
            </BtgPanel>
          </li>
        </ol>
      </BtgSection>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAuth } from '../auth/auth'
import { loginLocation, routeReturnPath } from '../auth/navigation'
import { authoringDraftPath, listAuthoringDrafts, type AuthoringDraftSummary } from '../authoring/authoring'
import BtgBadge from '../components/BtgBadge.vue'
import BtgButton from '../components/BtgButton.vue'
import BtgIcon from '../components/BtgIcon.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import BtgSection from '../components/BtgSection.vue'
import { useApplicationFormatters } from '../i18n/formatters'

type State = { kind: 'loading' } | { kind: 'ready'; drafts: AuthoringDraftSummary[] } | { kind: 'unavailable' }
const auth = useAuth()
const router = useRouter()
const { t } = useI18n()
const formatters = useApplicationFormatters()
const state = ref<State>({ kind: 'loading' })
let requestVersion = 0
let active = true

watch(
  () => auth.state.value,
  (session) => {
    const status = session.status
    if (status === 'authenticated') { void load(); return }
    requestVersion += 1
    state.value = { kind: 'loading' }
    if (status === 'unauthenticated') void router.replace(loginLocation(routeReturnPath(router.currentRoute.value)))
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
function statusLabel(status: AuthoringDraftSummary['status']) { return t(`authoring.entry.status.${status}`) }
function statusVariant(status: AuthoringDraftSummary['status']): 'success' | 'neutral' { return status === 'ACTIVE' ? 'success' : 'neutral' }
function createDraft() { void router.push('/authoring/new') }
</script>
