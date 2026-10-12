<template>
  <BtgPageContainer as="article" class="certificate-page" aria-labelledby="certificate-title">
    <BtgPanel v-if="state.kind === 'loading'" class="certificate-page__state" variant="subtle" role="status">
      <BtgPageTitle id="certificate-title" :title="t('certificates.private.loadingTitle')" :description="t('certificates.private.loadingDescription')" />
    </BtgPanel>
    <BtgPanel v-else-if="state.kind === 'missing'" class="certificate-page__state" variant="subtle">
      <BtgPageTitle id="certificate-title" :title="t('certificates.private.unavailableTitle')" :description="t('certificates.private.notFound')" />
    </BtgPanel>
    <BtgPanel v-else-if="state.kind === 'unavailable'" class="certificate-page__state" variant="subtle" role="alert">
      <BtgPageTitle id="certificate-title" :title="t('certificates.private.unavailableTitle')" :description="t('certificates.private.loadFailed')" />
      <BtgButton variant="secondary" @click="load">{{ t('common.actions.tryAgain') }}</BtgButton>
    </BtgPanel>

    <template v-else>
      <div class="certificate-page__header">
        <p class="certificate-page__eyebrow"><BtgIcon name="certificate" decorative />{{ t('certificates.private.eyebrow') }}</p>
        <BtgPageTitle id="certificate-title" focusable :title="state.certificate.achievement.title" />
        <BtgBadge :variant="state.certificate.status === 'ACTIVE' ? 'success' : 'danger'" :icon="state.certificate.status === 'ACTIVE' ? 'verified' : 'certificate'">{{ statusLabel(state.certificate.status) }}</BtgBadge>
        <p role="status">{{ statusDescription(state.certificate) }}</p>
      </div>

      <BtgMetadataStrip class="certificate-page__metadata" :items="metadata(state.certificate)" />

      <BtgSection>
        <template #heading>{{ t('certificates.private.achievement') }}</template>
        <BtgPanel><h3>{{ t('certificates.private.criteria') }}</h3><p>{{ state.certificate.achievement.criteria }}</p></BtgPanel>
      </BtgSection>

      <BtgSection>
        <template #heading>{{ t('certificates.private.details') }}</template>
        <BtgPanel class="certificate-page__identifier" padding="compact">
          <p>{{ t('certificates.private.identifier') }}</p>
          <code>{{ state.certificate.certificateId }}</code>
        </BtgPanel>
      </BtgSection>

      <div class="certificate-page__actions">
        <RouterLink class="btg-button btg-button--primary" :to="verificationPath(state.certificate.certificateId)"><BtgIcon name="verified" decorative />{{ t('certificates.private.openVerification') }}</RouterLink>
        <a v-if="state.certificate.openBadgePath" class="btg-button btg-button--secondary" :href="state.certificate.openBadgePath"><BtgIcon name="external" decorative />{{ t('certificates.private.openBadge') }}</a>
      </div>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAuth } from '../auth/auth'
import { APIProblemError } from '../api/client'
import BtgBadge from '../components/BtgBadge.vue'
import BtgButton from '../components/BtgButton.vue'
import BtgIcon from '../components/BtgIcon.vue'
import BtgMetadataStrip from '../components/BtgMetadataStrip.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import BtgSection from '../components/BtgSection.vue'
import { getLearnerCertificate, verificationPath, type LearnerCertificate } from '../courses/certificates'
import { InvalidCourseRouteError } from '../courses/courses'
import { useApplicationFormatters } from '../i18n/formatters'
import { languageLabel } from '../i18n/languages'

type State = { kind: 'loading' } | { kind: 'missing' } | { kind: 'unavailable' } | { kind: 'ready'; certificate: LearnerCertificate }
const route = useRoute()
const auth = useAuth()
const { t } = useI18n()
const { date } = useApplicationFormatters()
const state = ref<State>({ kind: 'loading' })
let active = true
let generation = 0

watch(() => route.params.certificateId, () => { void load() }, { immediate: true })
onBeforeUnmount(() => { active = false; generation += 1 })

async function load() {
  const id = typeof route.params.certificateId === 'string' ? route.params.certificateId : ''
  const request = ++generation
  state.value = { kind: 'loading' }
  try {
    const certificate = await getLearnerCertificate(id, auth)
    if (active && request === generation && id === route.params.certificateId) state.value = { kind: 'ready', certificate }
  } catch (error) {
    if (!active || request !== generation) return
    state.value = error instanceof InvalidCourseRouteError || (error instanceof APIProblemError && error.status === 404) ? { kind: 'missing' } : { kind: 'unavailable' }
  }
}
function metadata(certificate: LearnerCertificate) {
  return [
    { label: t('certificates.private.version'), value: certificate.achievement.version },
    { label: t('certificates.private.language'), value: languageLabel(certificate.achievement.language) },
    { label: t('certificates.private.issued'), value: date(certificate.issuedAt, 'date') },
    { label: t('certificates.private.issuer'), value: certificate.issuer.name },
  ]
}
function statusLabel(status: LearnerCertificate['status']) { return status === 'ACTIVE' ? t('certificates.private.active') : t('certificates.private.revoked') }
function statusDescription(certificate: LearnerCertificate) {
  return certificate.status === 'ACTIVE' || !certificate.revokedAt
    ? t('certificates.private.activeDescription')
    : t('certificates.private.revokedDescription', { date: date(certificate.revokedAt, 'date') })
}
</script>
