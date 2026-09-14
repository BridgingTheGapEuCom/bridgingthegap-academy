<template>
  <BtgPageContainer as="section" class="login-page" width="form" aria-labelledby="login-title">
    <div v-if="auth.state.value.status === 'bootstrapping'" class="login-page__state" role="status">
      <h1 id="login-title">Checking your session</h1>
      <p>One moment while we check whether you are already signed in.</p>
    </div>

    <div v-else-if="auth.state.value.status === 'authenticated'" class="login-page__state">
      <h1 id="login-title">You are already signed in.</h1>
      <p>Continue to the Academy home page.</p>
      <BtgButton variant="secondary" @click="continueHome">Continue to home</BtgButton>
    </div>

    <div v-else-if="auth.state.value.status === 'unavailable'" class="login-page__state">
      <h1 id="login-title">Sign in unavailable</h1>
      <p>We couldn’t check your session right now. Please try again.</p>
      <BtgButton variant="secondary" @click="retryBootstrap">Try again</BtgButton>
    </div>

    <div v-else class="login-page__content">
      <p class="login-page__eyebrow">Bridging the Gap Academy</p>
      <h1 id="login-title">Sign in</h1>
      <p class="login-page__intro">Use your Academy email address and password to continue.</p>

      <form class="login-page__form" aria-labelledby="login-title" :aria-busy="submitting" novalidate @submit.prevent="submit">
        <div v-if="formError" ref="formErrorElement" class="login-page__form-error" role="alert" tabindex="-1">
          {{ formError }}
        </div>

        <BtgFormField label="Email" required :error="emailError" v-slot="{ controlId, describedBy, invalid }">
          <BtgTextInput
            ref="emailInput"
            v-model="email"
            :id="controlId"
            :aria-describedby="describedBy"
            :invalid="invalid"
            type="email"
            autocomplete="username"
            required
          />
        </BtgFormField>

        <BtgFormField label="Password" required :error="passwordError" v-slot="{ controlId, describedBy, invalid }">
          <BtgTextInput
            ref="passwordInput"
            v-model="password"
            :id="controlId"
            :aria-describedby="describedBy"
            :invalid="invalid"
            type="password"
            autocomplete="current-password"
            required
          />
        </BtgFormField>

        <div class="login-page__actions">
          <BtgButton type="submit" :disabled="submitting">{{ submitting ? 'Signing in…' : 'Sign in' }}</BtgButton>
        </div>
      </form>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../auth/auth'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgTextInput from '../components/BtgTextInput.vue'

type FocusableInput = { focus: () => void }

const auth = useAuth()
const router = useRouter()
const email = ref('')
const password = ref('')
const emailError = ref<string>()
const passwordError = ref<string>()
const formError = ref<string>()
const submitting = ref(false)
const emailInput = ref<FocusableInput>()
const passwordInput = ref<FocusableInput>()
const formErrorElement = ref<HTMLElement>()

async function submit() {
  if (submitting.value) return

  emailError.value = undefined
  passwordError.value = undefined
  formError.value = undefined

  const submittedEmail = email.value.trim()
  if (!submittedEmail) emailError.value = 'Enter your email address.'
  if (!password.value) passwordError.value = 'Enter your password.'
  if (emailError.value || passwordError.value) {
    await nextTick()
    if (emailError.value) emailInput.value?.focus()
    else passwordInput.value?.focus()
    return
  }

  submitting.value = true
  const outcome = await auth.login(submittedEmail, password.value)
  password.value = ''
  submitting.value = false

  if (outcome.kind === 'authenticated') {
    await router.push('/')
    return
  }

  if (outcome.kind === 'invalid-credentials') formError.value = 'The email or password is incorrect.'
  else if (outcome.kind === 'rate-limited') formError.value = rateLimitMessage(outcome.retryAfterSeconds)
  else formError.value = 'We couldn’t sign you in right now. Please try again.'
  await nextTick()
  formErrorElement.value?.focus()
}

async function retryBootstrap() {
  await auth.bootstrapSession()
}

function continueHome() {
  void router.push('/')
}

function rateLimitMessage(retryAfterSeconds?: number): string {
  if (retryAfterSeconds && retryAfterSeconds > 0) return `Too many sign-in attempts. Please try again in about ${retryAfterSeconds} seconds.`
  return 'Too many sign-in attempts. Please try again shortly.'
}
</script>
