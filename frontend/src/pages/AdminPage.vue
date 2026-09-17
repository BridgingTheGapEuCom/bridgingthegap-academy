<template>
  <BtgPageContainer as="section" class="admin-page" width="reading" aria-labelledby="admin-title">
    <p class="sr-only" role="status">{{ accessAnnouncement }}</p>
    <div v-if="auth.state.value.status === 'bootstrapping'" class="admin-page__state" role="status">
      <h1 id="admin-title">Checking your session</h1>
      <p>One moment while we check whether you are signed in.</p>
    </div>

    <div v-else-if="auth.state.value.status === 'unavailable'" class="admin-page__state">
      <h1 id="admin-title">Administration unavailable</h1>
      <p>We couldn’t check your session right now. Please try again.</p>
      <BtgButton variant="secondary" @click="retryBootstrap">Try again</BtgButton>
    </div>

    <div v-else-if="auth.state.value.status === 'unauthenticated'" class="admin-page__state" role="status">
      <h1 id="admin-title">Sign in required</h1>
      <p>Taking you to sign in.</p>
    </div>

    <div v-else-if="access.kind === 'checking'" class="admin-page__state" role="status">
      <h1 id="admin-title">Checking access…</h1>
      <p>One moment while we confirm access to Administration.</p>
    </div>

    <div v-else-if="access.kind === 'authorized'" class="admin-page__content">
      <p class="admin-page__eyebrow">Bridging the Gap Academy</p>
      <h1 id="admin-title">Administration</h1>
      <p class="admin-page__intro">Administrator access is available for this Academy instance.</p>
      <p class="admin-page__status">System status: OK</p>
      <div class="admin-page__actions">
        <BtgButton variant="secondary" @click="goHome">Return home</BtgButton>
      </div>
    </div>

    <div v-else-if="access.kind === 'forbidden'" class="admin-page__state">
      <h1 id="admin-title">Access denied</h1>
      <p>Your account does not have permission to access Administration.</p>
      <BtgButton variant="secondary" @click="goHome">Return home</BtgButton>
    </div>

    <div v-else class="admin-page__state">
      <h1 id="admin-title">Administration unavailable</h1>
      <p>Administration is temporarily unavailable. Please try again.</p>
      <BtgButton variant="secondary" @click="retryAccess">Try again</BtgButton>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { checkAdminAccess, type AdminAccessOutcome } from '../admin/adminAccess'
import { useAuth } from '../auth/auth'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'

type AccessState = AdminAccessOutcome | { kind: 'checking' }

const auth = useAuth()
const router = useRouter()
const access = ref<AccessState>({ kind: 'checking' })
const accessAnnouncement = computed(() => {
  if (auth.state.value.status !== 'authenticated') return ''
  if (access.value.kind === 'authorized') return 'Administration access confirmed.'
  if (access.value.kind === 'forbidden') return 'Access denied.'
  if (access.value.kind === 'unavailable') return 'Administration is temporarily unavailable.'
  return ''
})
let requestVersion = 0
let active = true

watch(
  () => auth.state.value,
  (current) => {
    const version = ++requestVersion
    if (current.status === 'authenticated') {
      void checkAccess(version)
      return
    }
    access.value = { kind: 'checking' }
    if (current.status === 'unauthenticated') void router.replace('/login')
  },
  { immediate: true },
)

onBeforeUnmount(() => { active = false })

async function checkAccess(version = ++requestVersion) {
  if (auth.state.value.status !== 'authenticated') return
  access.value = { kind: 'checking' }
  const outcome = await checkAdminAccess(auth)
  if (!active || version !== requestVersion) return

  if (outcome.kind === 'unauthenticated') {
    void router.replace('/login')
    return
  }
  access.value = outcome
}

async function retryBootstrap() {
  await auth.bootstrapSession()
}

function retryAccess() {
  void checkAccess()
}

function goHome() {
  void router.push('/')
}
</script>
