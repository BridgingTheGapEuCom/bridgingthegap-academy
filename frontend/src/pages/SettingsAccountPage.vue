<template>
  <SettingsShell :title="t('settings.title')" title-id="settings-account-title" :navigation-label="t('settings.navigationLabel')" :items="navigationItems">
    <BtgPageTitle id="settings-account-title" :title="t('settings.account.title')" :description="t('settings.account.description')" />
    <BtgSection heading-id="settings-account-session-title" class="settings-account__section">
      <template #heading>{{ t('settings.account.session') }}</template>
      <p>{{ t('settings.account.sessionDescription') }}</p>
      <BtgMetadataStrip :items="sessionMetadata" layout="default" />
    </BtgSection>
  </SettingsShell>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuth } from '../auth/auth'
import BtgMetadataStrip from '../components/BtgMetadataStrip.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgSection from '../components/BtgSection.vue'
import SettingsShell from '../components/SettingsShell.vue'
import { useApplicationFormatters } from '../i18n/formatters'

const { t } = useI18n()
const { date } = useApplicationFormatters()
const auth = useAuth()
const navigationItems = computed(() => [
  { to: '/settings/preferences', label: t('settings.preferences') },
  { to: '/settings/account', label: t('settings.account.title') },
])
const sessionMetadata = computed(() => {
  const expiresAt = auth.state.value.status === 'authenticated' ? auth.state.value.expiresAt : undefined
  const formattedExpiry = expiresAt ? date(expiresAt, 'dateTime') : ''
  return [{ label: t('settings.account.sessionExpires'), value: formattedExpiry || t('settings.account.notAvailable') }]
})
</script>

<style scoped>
.settings-account__section { max-inline-size: 48rem; }
.settings-account__section > p { margin: 0; color: var(--academy-text-secondary); }
</style>
