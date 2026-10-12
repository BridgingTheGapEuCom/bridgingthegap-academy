<template>
  <BtgPageContainer as="section" class="login-page" aria-labelledby="login-title">
    <BtgPanel v-if="auth.state.value.status === 'bootstrapping'" class="login-page__state" role="status">
      <BtgPageTitle id="login-title" :title="t('auth.checking.title')" :description="t('auth.checking.description')" />
    </BtgPanel>

    <BtgPanel v-else-if="auth.state.value.status === 'authenticated'" class="login-page__state">
      <BtgPageTitle id="login-title" :title="t('auth.authenticated.title')" :description="t('auth.authenticated.description')" />
      <BtgButton variant="secondary" @click="continueHome">{{ t('auth.authenticated.action') }}</BtgButton>
    </BtgPanel>

    <BtgPanel v-else-if="auth.state.value.status === 'unavailable'" class="login-page__state">
      <BtgPageTitle id="login-title" :title="t('auth.unavailable.title')" :description="t('auth.unavailable.description')" />
      <BtgButton variant="secondary" @click="retryBootstrap">{{ t('auth.unavailable.retry') }}</BtgButton>
    </BtgPanel>

    <div v-else class="login-page__content">
      <BtgPageTitle id="login-title" :title="t('auth.login.title')" :description="t('auth.login.description')" />

      <BtgPanel
        v-if="auth.state.value.reason === 'session-expired'"
        class="login-page__session-expired"
        variant="subtle"
        role="status"
      >
        <h2>{{ t('auth.sessionExpired.title') }}</h2>
        <p>{{ t('auth.sessionExpired.description') }}</p>
      </BtgPanel>

      <BtgPanel as="section">
        <form class="login-page__form" aria-labelledby="login-title" :aria-busy="submitting" novalidate @submit.prevent="submit">
          <BtgPanel v-if="formError" variant="subtle" padding="compact" role="alert">
            <p ref="formErrorElement" tabindex="-1">{{ formError }}</p>
          </BtgPanel>

          <BtgFormField :label="t('auth.login.email')" required :error="emailError" v-slot="{ controlId, describedBy, invalid }">
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

          <BtgFormField :label="t('auth.login.password')" required :error="passwordError" v-slot="{ controlId, describedBy, invalid }">
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
            <BtgButton type="submit" :disabled="submitting">{{ submitting ? t('auth.login.submitting') : t('auth.login.submit') }}</BtgButton>
          </div>
        </form>
      </BtgPanel>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '../auth/auth'
import { safeInternalReturnPath } from '../auth/navigation'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import BtgTextInput from '../components/BtgTextInput.vue'
import { useApplicationFormatters } from '../i18n/formatters'

type FocusableInput = { focus: () => void }
type ValidationCode = 'email-required' | 'password-required'

const auth = useAuth()
const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const { relativeTime } = useApplicationFormatters()
const email = ref('')
const password = ref('')
const emailErrorCode = ref<ValidationCode>()
const passwordErrorCode = ref<ValidationCode>()
const emailError = computed(() => emailErrorCode.value === 'email-required' ? t('auth.validation.emailRequired') : undefined)
const passwordError = computed(() => passwordErrorCode.value === 'password-required' ? t('auth.validation.passwordRequired') : undefined)
const formError = ref<string>()
const submitting = ref(false)
const emailInput = ref<FocusableInput>()
const passwordInput = ref<FocusableInput>()
const formErrorElement = ref<HTMLElement>()
let active = true

onBeforeUnmount(() => {
  active = false
  password.value = ''
})

async function submit() {
  if (submitting.value) return

  emailErrorCode.value = undefined
  passwordErrorCode.value = undefined
  formError.value = undefined

  const submittedEmail = email.value.trim()
  if (!submittedEmail) emailErrorCode.value = 'email-required'
  if (!password.value) passwordErrorCode.value = 'password-required'
  if (emailError.value || passwordError.value) {
    await nextTick()
    if (emailError.value) emailInput.value?.focus()
    else passwordInput.value?.focus()
    return
  }

  submitting.value = true
  let outcome: Awaited<ReturnType<typeof auth.login>>
  try {
    outcome = await auth.login(submittedEmail, password.value)
  } catch {
    outcome = { kind: 'unavailable' }
  } finally {
    password.value = ''
    submitting.value = false
  }
  if (!active) return

  if (outcome.kind === 'authenticated') {
    await router.push(returnPath())
    return
  }

  if (outcome.kind === 'invalid-credentials') formError.value = t('auth.errors.invalidCredentials')
  else if (outcome.kind === 'rate-limited') formError.value = rateLimitMessage(outcome.retryAfterSeconds)
  else formError.value = t('auth.errors.unavailable')
  await nextTick()
  formErrorElement.value?.focus()
}

async function retryBootstrap() {
  await auth.bootstrapSession()
}

function continueHome() {
  void router.push(returnPath())
}

function returnPath(): string {
  return safeInternalReturnPath(route.query.returnTo)
}

function rateLimitMessage(retryAfterSeconds?: number): string {
  if (retryAfterSeconds && retryAfterSeconds > 0) {
    return t('auth.errors.rateLimited', { time: relativeTime(retryAfterSeconds, 'second') })
  }
  return t('auth.errors.rateLimitedSoon')
}
</script>
