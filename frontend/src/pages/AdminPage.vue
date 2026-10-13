<template>
  <BtgPageContainer as="section" class="admin-page" aria-labelledby="admin-title">
    <p class="sr-only" role="status">{{ accessAnnouncement }}</p>

    <BtgPanel v-if="auth.state.value.status === 'bootstrapping'" class="admin-page__state" variant="subtle" role="status">
      <BtgPageTitle id="admin-title" :title="t('admin.access.checkingSessionTitle')" :description="t('admin.access.checkingSessionDescription')" />
    </BtgPanel>
    <BtgPanel v-else-if="auth.state.value.status === 'unavailable'" class="admin-page__state" variant="subtle" role="alert">
      <BtgPageTitle id="admin-title" :title="t('admin.access.unavailableTitle')" :description="t('admin.access.sessionUnavailable')" />
      <BtgButton variant="secondary" @click="retryBootstrap">{{ t('common.actions.tryAgain') }}</BtgButton>
    </BtgPanel>
    <BtgPanel v-else-if="auth.state.value.status === 'unauthenticated'" class="admin-page__state" variant="subtle" role="status">
      <BtgPageTitle id="admin-title" :title="t('admin.access.signInTitle')" :description="t('admin.access.signInDescription')" />
    </BtgPanel>
    <BtgPanel v-else-if="access.kind === 'checking'" class="admin-page__state" variant="subtle" role="status">
      <BtgPageTitle id="admin-title" :title="t('admin.access.checkingTitle')" :description="t('admin.access.checkingDescription')" />
    </BtgPanel>

    <div v-else-if="access.kind === 'authorized'" class="admin-page__content">
      <BtgPageTitle id="admin-title" :title="t('admin.landing.title')" :description="t('admin.landing.helper')" />
      <BtgBadge variant="success" icon="verified">{{ t('admin.landing.status') }}</BtgBadge>
      <div class="admin-page__destinations">
        <BtgPanel as="article" class="admin-page__destination">
          <BtgIcon name="import" size="large" decorative />
          <h2>{{ t('admin.landing.courseImportTitle') }}</h2>
          <p>{{ t('admin.landing.courseImportDescription') }}</p>
          <RouterLink class="btg-button btg-button--secondary" to="/admin/course-import"><BtgIcon name="import" decorative />{{ t('admin.landing.courseImportAction') }}</RouterLink>
        </BtgPanel>
        <BtgPanel as="article" class="admin-page__destination">
          <BtgIcon name="plugin" size="large" decorative />
          <h2>{{ t('admin.landing.pluginsTitle') }}</h2>
          <p>{{ t('admin.landing.pluginsDescription') }}</p>
          <RouterLink class="btg-button btg-button--secondary" to="/admin/plugins"><BtgIcon name="plugin" decorative />{{ t('admin.landing.pluginsAction') }}</RouterLink>
        </BtgPanel>
      </div>
      <div class="admin-page__actions"><BtgButton variant="tertiary" leading-icon="arrowLeft" @click="goHome">{{ t('admin.landing.returnHome') }}</BtgButton></div>
    </div>

    <BtgPanel v-else-if="access.kind === 'forbidden'" class="admin-page__state" variant="subtle">
      <BtgPageTitle id="admin-title" :title="t('admin.access.deniedTitle')" :description="t('admin.access.deniedDescription')" />
      <BtgButton variant="secondary" @click="goHome">{{ t('admin.landing.returnHome') }}</BtgButton>
    </BtgPanel>
    <BtgPanel v-else class="admin-page__state" variant="subtle" role="alert">
      <BtgPageTitle id="admin-title" :title="t('admin.access.unavailableTitle')" :description="t('admin.access.unavailable')" />
      <BtgButton variant="secondary" @click="retryAccess">{{ t('common.actions.tryAgain') }}</BtgButton>
    </BtgPanel>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { checkAdminAccess, type AdminAccessOutcome } from '../admin/adminAccess'
import { useAuth } from '../auth/auth'
import BtgBadge from '../components/BtgBadge.vue'
import BtgButton from '../components/BtgButton.vue'
import BtgIcon from '../components/BtgIcon.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'

type AccessState = AdminAccessOutcome | { kind: 'checking' }
const auth = useAuth()
const router = useRouter()
const { t } = useI18n()
const access = ref<AccessState>({ kind: 'checking' })
const accessAnnouncement = computed(() => {
  if (auth.state.value.status !== 'authenticated') return ''
  if (access.value.kind === 'authorized') return t('admin.access.confirmedAnnouncement')
  if (access.value.kind === 'forbidden') return t('admin.access.deniedAnnouncement')
  if (access.value.kind === 'unavailable') return t('admin.access.unavailableAnnouncement')
  return ''
})
let requestVersion = 0
let active = true

watch(() => auth.state.value, (current) => {
  const version = ++requestVersion
  if (current.status === 'authenticated') { void checkAccess(version); return }
  access.value = { kind: 'checking' }
  if (current.status === 'unauthenticated') void router.replace('/login')
}, { immediate: true })
onBeforeUnmount(() => { active = false })

async function checkAccess(version = ++requestVersion) {
  if (auth.state.value.status !== 'authenticated') return
  access.value = { kind: 'checking' }
  const outcome = await checkAdminAccess(auth)
  if (!active || version !== requestVersion) return
  if (outcome.kind === 'unauthenticated') { void router.replace('/login'); return }
  access.value = outcome
}
async function retryBootstrap() { await auth.bootstrapSession() }
function retryAccess() { void checkAccess() }
function goHome() { void router.push('/') }
</script>
