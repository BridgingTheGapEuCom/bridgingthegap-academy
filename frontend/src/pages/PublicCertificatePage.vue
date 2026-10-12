<template>
  <BtgPageContainer as="article" class="certificate-page certificate-page--public" aria-labelledby="verification-title">
    <BtgPanel v-if="state.kind === 'loading'" class="certificate-page__state" variant="subtle" role="status">
      <BtgPageTitle id="verification-title" :title="t('certificates.verify.loadingTitle')" :description="t('certificates.verify.loadingDescription')" />
    </BtgPanel>
    <BtgPanel v-else-if="state.kind === 'missing'" class="certificate-page__state" variant="subtle" role="status">
      <BtgPageTitle id="verification-title" :title="t('certificates.verify.notFoundTitle')" :description="t('certificates.verify.notFound')" />
    </BtgPanel>
    <BtgPanel v-else-if="state.kind === 'unavailable'" class="certificate-page__state" variant="subtle" role="alert">
      <BtgPageTitle id="verification-title" :title="t('certificates.verify.unavailableTitle')" :description="t('certificates.verify.unavailable')" />
      <BtgButton variant="secondary" @click="load">{{ t('common.actions.tryAgain') }}</BtgButton>
    </BtgPanel>

    <template v-else>
      <div class="certificate-page__header">
        <p class="certificate-page__eyebrow"><BtgIcon name="verified" decorative />{{ t('certificates.verify.eyebrow') }}</p>
        <BtgPageTitle id="verification-title" focusable :title="t('certificates.verify.title')" :description="t('certificates.verify.helper')" />
      </div>

      <BtgPanel class="certificate-page__result" :variant="state.certificate.status === 'ACTIVE' ? 'default' : 'subtle'" role="status" aria-live="polite">
        <BtgBadge :variant="state.certificate.status === 'ACTIVE' ? 'success' : 'danger'" :icon="state.certificate.status === 'ACTIVE' ? 'verified' : 'certificate'">{{ statusLabel(state.certificate.status) }}</BtgBadge>
        <h2>{{ state.certificate.achievement.title }}</h2>
        <p>{{ statusDescription(state.certificate) }}</p>
      </BtgPanel>

      <BtgSection>
        <template #heading>{{ t('certificates.verify.achievement') }}</template>
        <BtgMetadataStrip class="certificate-page__metadata" :items="metadata(state.certificate)" />
        <BtgPanel><h3>{{ t('certificates.verify.criteria') }}</h3><p>{{ state.certificate.achievement.criteria }}</p></BtgPanel>
      </BtgSection>

      <BtgSection>
        <template #heading>{{ t('certificates.verify.record') }}</template>
        <BtgPanel class="certificate-page__identifier" padding="compact">
          <p>{{ t('certificates.verify.identifier') }}</p>
          <code>{{ state.certificate.certificateId }}</code>
        </BtgPanel>
      </BtgSection>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { APIProblemError } from '../api/client'
import BtgBadge from '../components/BtgBadge.vue'
import BtgButton from '../components/BtgButton.vue'
import BtgIcon from '../components/BtgIcon.vue'
import BtgMetadataStrip from '../components/BtgMetadataStrip.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import BtgSection from '../components/BtgSection.vue'
import { getPublicCertificate, type PublicCertificate } from '../courses/certificates'
import { InvalidCourseRouteError } from '../courses/courses'
import { useApplicationFormatters } from '../i18n/formatters'
import { languageLabel } from '../i18n/languages'

type State = { kind: 'loading' } | { kind: 'missing' } | { kind: 'unavailable' } | { kind: 'ready'; certificate: PublicCertificate }
const route = useRoute()
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
    const certificate = await getPublicCertificate(id)
    if (active && request === generation && id === route.params.certificateId) state.value = { kind: 'ready', certificate }
  } catch (error) {
    if (!active || request !== generation) return
    state.value = error instanceof InvalidCourseRouteError || (error instanceof APIProblemError && error.status === 404) ? { kind: 'missing' } : { kind: 'unavailable' }
  }
}
function metadata(certificate: PublicCertificate) {
  return [
    { label: t('certificates.verify.version'), value: certificate.achievement.version },
    { label: t('certificates.verify.language'), value: languageLabel(certificate.achievement.language) },
    { label: t('certificates.verify.issued'), value: date(certificate.issuedAt, 'date') },
    { label: t('certificates.verify.issuer'), value: certificate.issuer.name },
  ]
}
function statusLabel(status: PublicCertificate['status']) { return status === 'ACTIVE' ? t('certificates.verify.valid') : t('certificates.verify.revoked') }
function statusDescription(certificate: PublicCertificate) {
  return certificate.status === 'ACTIVE' || !certificate.revokedAt
    ? t('certificates.verify.validDescription', { issuer: certificate.issuer.name })
    : t('certificates.verify.revokedDescription', { date: date(certificate.revokedAt, 'date') })
}
</script>
