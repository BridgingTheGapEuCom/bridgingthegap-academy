<template>
  <div class="site-session" :aria-busy="auth.state.value.status === 'bootstrapping' || undefined">
    <p v-if="auth.state.value.status === 'bootstrapping'" role="status">Checking session…</p>
    <template v-else-if="auth.state.value.status === 'authenticated'">
      <p aria-label="Authentication status">Signed in</p>
      <BtgButton variant="quiet" :disabled="signingOut" @click="signOut">{{ signingOut ? 'Signing out…' : 'Sign out' }}</BtgButton>
    </template>
    <RouterLink v-else to="/login">Sign in</RouterLink>
    <p v-if="auth.state.value.status === 'unavailable'" role="status">Session check unavailable.</p>
    <p v-if="error" ref="errorElement" class="site-session__error" role="alert" tabindex="-1">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../auth/auth'
import BtgButton from './BtgButton.vue'

const auth = useAuth()
const router = useRouter()
const signingOut = ref(false)
const error = ref<string>()
const errorElement = ref<HTMLElement>()

async function signOut() {
  if (signingOut.value) return
  error.value = undefined
  signingOut.value = true
  try {
    const outcome = await auth.logout()
    if (outcome.kind === 'unauthenticated') {
      await router.replace('/login')
      return
    }
    error.value = 'We couldn’t sign you out right now. Please try again.'
    await nextTick()
    errorElement.value?.focus()
  } catch {
    error.value = 'We couldn’t sign you out right now. Please try again.'
    await nextTick()
    errorElement.value?.focus()
  } finally {
    signingOut.value = false
  }
}
</script>
