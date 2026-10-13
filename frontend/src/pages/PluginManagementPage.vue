<template>
  <BtgPageContainer as="section" class="plugin-management" aria-labelledby="plugin-management-title">
    <p class="sr-only" role="status" aria-live="polite">{{ announcement }}</p>
    <p class="btg-page-eyebrow">{{ t('admin.context') }}</p>
    <BtgPageTitle id="plugin-management-title" :title="t('admin.plugins.title')" :description="t('admin.plugins.helper')" />

    <BtgPanel v-if="state === 'loading'" variant="subtle" role="status"><p>{{ t('admin.plugins.loading') }}</p></BtgPanel>
    <BtgPanel v-else-if="state === 'forbidden'" variant="subtle"><h2>{{ t('admin.plugins.deniedTitle') }}</h2><p>{{ t('admin.plugins.deniedDescription') }}</p></BtgPanel>
    <BtgPanel v-else-if="state === 'error'" variant="subtle" role="alert"><h2>{{ t('admin.plugins.unavailableTitle') }}</h2><p>{{ t('admin.plugins.unavailableDescription') }}</p><BtgButton variant="secondary" @click="load">{{ t('common.actions.tryAgain') }}</BtgButton></BtgPanel>

    <template v-else>
      <BtgPanel v-if="error" variant="subtle" role="alert"><p>{{ error }}</p></BtgPanel>

      <BtgSection heading-id="plugin-install-title">
        <template #heading>{{ t('admin.plugins.installTitle') }}</template>
        <p>{{ t('admin.plugins.installDescription') }}</p>
        <BtgPanel>
          <form class="plugin-management__form" @submit.prevent="install">
            <BtgFileField ref="packageField" v-model="packageFile" :label="t('admin.plugins.fileLabel')" :description="t('admin.plugins.fileHelper')" :empty-text="t('admin.plugins.noFile')" :remove-label="t('admin.plugins.removeFile')" :selected-description="packageFile ? bytes(packageFile.size) : undefined" accept=".zip,application/zip" required :disabled="Boolean(busy)" @update:model-value="selectPackage" />
            <div class="plugin-management__actions"><BtgButton type="submit" leading-icon="plugin" :loading="busy === 'install'" :disabled="Boolean(busy) || !packageFile">{{ busy === 'install' ? t('admin.plugins.installing') : t('admin.plugins.install') }}</BtgButton></div>
          </form>
        </BtgPanel>
      </BtgSection>

      <BtgSection heading-id="plugin-releases-title">
        <template #heading>{{ t('admin.plugins.releasesTitle') }}</template>
        <p>{{ t('admin.plugins.releasesDescription') }}</p>
        <BtgPanel v-if="!plugins.length" variant="subtle"><p>{{ t('admin.plugins.noReleases') }}</p></BtgPanel>
        <ol v-else class="plugin-management__list" :aria-label="t('admin.plugins.releaseListLabel')">
          <li v-for="release in plugins" :key="releaseKey(release)">
            <BtgPanel as="article" class="plugin-management__release">
              <header class="plugin-management__release-header">
                <div><h3>{{ release.name }} <span>{{ release.version }}</span></h3><p>{{ release.pluginId }}</p></div>
                <div class="plugin-management__badges">
                  <BtgBadge :variant="trustVariant(release)">{{ trustLabel(release) }}</BtgBadge>
                  <BtgBadge :variant="release.enabled ? 'success' : 'neutral'">{{ release.enabled ? t('admin.plugins.enabled') : t('admin.plugins.disabled') }}</BtgBadge>
                </div>
              </header>
              <BtgMetadataStrip :items="releaseMetadata(release)" />
              <BtgPanel v-if="release.validationStatus === 'SIGNATURE_INVALID'" variant="subtle"><p>{{ t('admin.plugins.invalidSignatureExplanation') }}</p></BtgPanel>
              <BtgPanel v-else-if="release.currentTrust === 'UNKNOWN'" variant="subtle"><p>{{ t('admin.plugins.unknownTrustExplanation') }}</p></BtgPanel>
              <p>{{ t('admin.plugins.widgets') }}: {{ release.entrypoints.length ? release.entrypoints.map((entrypoint) => `${entrypoint.name} (${entrypoint.type})`).join(', ') : t('admin.plugins.none') }}</p>
              <details @toggle="openDetail(release, $event)"><summary>{{ t('admin.plugins.releaseDetails') }}</summary>
                <p v-if="detailLoading === releaseKey(release)" role="status">{{ t('admin.plugins.loadingDetails') }}</p>
                <div v-else-if="details[releaseKey(release)]" class="plugin-management__details">
                  <p><strong>{{ t('admin.plugins.publisher') }}:</strong> {{ details[releaseKey(release)]!.publisherName }}</p>
                  <p v-if="details[releaseKey(release)]!.homepage"><a :href="details[releaseKey(release)]!.homepage" target="_blank" rel="noopener noreferrer">{{ t('admin.plugins.homepage') }}</a></p>
                  <p>{{ details[releaseKey(release)]!.description }}</p>
                  <ul><li v-for="entrypoint in details[releaseKey(release)]!.entrypoints" :key="entrypoint.id"><strong>{{ entrypoint.name }}</strong> · {{ entrypoint.id }} · {{ entrypoint.type }}</li></ul>
                </div>
              </details>
              <div class="plugin-management__actions" :aria-label="t('admin.plugins.actionsLabel', { name: release.name, version: release.version })">
                <BtgButton v-if="release.approvalState !== 'ACTIVE'" variant="secondary" :disabled="Boolean(busy) || release.validationStatus === 'SIGNATURE_INVALID'" @click="approve(release)">{{ t('admin.plugins.approve') }}</BtgButton>
                <BtgButton v-else variant="secondary" :disabled="Boolean(busy)" @click="requestConfirmation('revoke', release)">{{ t('admin.plugins.revoke') }}</BtgButton>
                <BtgButton v-if="!release.enabled" leading-icon="power" :disabled="Boolean(busy) || !release.executionPermitted" @click="setEnabled(release, true)">{{ t('admin.plugins.enable') }}</BtgButton>
                <BtgButton v-else variant="secondary" leading-icon="power" :disabled="Boolean(busy)" @click="requestConfirmation('disable', release)">{{ t('admin.plugins.disable') }}</BtgButton>
              </div>
              <p v-if="release.enabled || release.approvalState === 'ACTIVE'">{{ t('admin.plugins.affectedWidgets') }}</p>
            </BtgPanel>
          </li>
        </ol>
      </BtgSection>

      <BtgSection heading-id="plugin-keys-title">
        <template #heading>{{ t('admin.plugins.keysTitle') }}</template>
        <p>{{ t('admin.plugins.keysDescription') }}</p>
        <BtgPanel>
          <form class="plugin-management__key-form" @submit.prevent="createKey">
            <BtgFormField :label="t('admin.plugins.keyID')" required v-slot="{ controlId, describedBy, invalid }"><BtgTextInput :id="controlId" v-model="keyID" :aria-describedby="describedBy" :invalid="invalid" :disabled="Boolean(busy)" /></BtgFormField>
            <BtgFormField :label="t('admin.plugins.keyPurpose')" required v-slot="{ controlId, describedBy, invalid }"><BtgSelect :id="controlId" v-model="keyPurpose" :aria-describedby="describedBy" :invalid="invalid" :disabled="Boolean(busy)"><option value="BTG_OWNED_SIGNING">{{ t('admin.plugins.ownedSigning') }}</option><option value="BTG_APPROVAL_SIGNING">{{ t('admin.plugins.approvalSigning') }}</option></BtgSelect></BtgFormField>
            <BtgFormField :label="t('admin.plugins.publicKey')" required :error="keyError" v-slot="{ controlId, describedBy, invalid }"><BtgTextarea :id="controlId" v-model="publicKey" :aria-describedby="describedBy" :invalid="invalid" :disabled="Boolean(busy)" spellcheck="false" /></BtgFormField>
            <BtgFormField v-if="keyPurpose === 'BTG_OWNED_SIGNING'" :label="t('admin.plugins.allowedPluginIDs')" :description="t('admin.plugins.allowedPluginIDsHelper')" required :error="allowListError" v-slot="{ controlId, describedBy, invalid }"><BtgTextarea :id="controlId" v-model="allowedPluginIDs" :aria-describedby="describedBy" :invalid="invalid" :disabled="Boolean(busy)" /></BtgFormField>
            <p v-else>{{ t('admin.plugins.approvalKeyExplanation') }}</p>
            <div class="plugin-management__actions"><BtgButton type="submit" leading-icon="key" :loading="busy === 'key'" :disabled="Boolean(busy) || Boolean(keyError) || Boolean(allowListError)">{{ busy === 'key' ? t('admin.plugins.addingKey') : t('admin.plugins.addKey') }}</BtgButton></div>
          </form>
        </BtgPanel>
        <BtgPanel v-if="!keys.length" variant="subtle"><p>{{ t('admin.plugins.noKeys') }}</p></BtgPanel>
        <ul v-else class="plugin-management__list" :aria-label="t('admin.plugins.keyListLabel')">
          <li v-for="key in keys" :key="key.keyId"><BtgPanel as="article"><header class="plugin-management__release-header"><h3>{{ key.keyId }}</h3><BtgBadge :variant="key.enabled ? 'success' : 'neutral'">{{ key.enabled ? t('admin.plugins.enabled') : t('admin.plugins.disabled') }}</BtgBadge></header><BtgMetadataStrip :items="keyMetadata(key)" /><div class="plugin-management__actions"><BtgButton v-if="!key.enabled" leading-icon="key" :disabled="Boolean(busy)" @click="setKeyEnabled(key, true)">{{ t('admin.plugins.enableKey') }}</BtgButton><BtgButton v-else variant="secondary" leading-icon="key" :disabled="Boolean(busy)" @click="setKeyEnabled(key, false)">{{ t('admin.plugins.disableKey') }}</BtgButton></div><p>{{ t('admin.plugins.keyEffect') }}</p></BtgPanel></li>
        </ul>
      </BtgSection>
    </template>

    <BtgDialog v-model:open="confirmationOpen" :show-trigger="false" :title="confirmationTitle" :description="confirmationDescription">
      <template #footer><BtgButton variant="secondary" @click="confirmationOpen = false">{{ t('admin.plugins.cancelConfirmation') }}</BtgButton><BtgButton variant="danger-secondary" :loading="busy === 'release'" @click="confirmLifecycle">{{ confirmationAction }}</BtgButton></template>
    </BtgDialog>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BtgBadge from '../components/BtgBadge.vue'
import BtgButton from '../components/BtgButton.vue'
import BtgDialog from '../components/BtgDialog.vue'
import BtgFileField from '../components/BtgFileField.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgMetadataStrip from '../components/BtgMetadataStrip.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import BtgSection from '../components/BtgSection.vue'
import BtgSelect from '../components/BtgSelect.vue'
import BtgTextInput from '../components/BtgTextInput.vue'
import BtgTextarea from '../components/BtgTextarea.vue'
import { useApplicationFormatters } from '../i18n/formatters'
import { addPluginKey, approvePlugin, getPlugin, installPlugin, isPluginManagementForbidden, listPluginKeys, listPlugins, pluginManagementError, revokePluginApproval, setPluginEnabled, setPluginKeyEnabled, type PluginRelease, type PluginVerificationKey } from '../plugins/management'

type PageState = 'loading' | 'ready' | 'forbidden' | 'error'
type Confirmation = { kind: 'revoke' | 'disable'; release: PluginRelease }
const { t } = useI18n()
const { bytes, date } = useApplicationFormatters()
const state = ref<PageState>('loading')
const plugins = ref<PluginRelease[]>([])
const keys = ref<PluginVerificationKey[]>([])
const details = ref<Record<string, PluginRelease>>({})
const detailLoading = ref('')
const packageFile = ref<File>()
const packageField = ref<InstanceType<typeof BtgFileField>>()
const announcement = ref('')
const error = ref('')
const busy = ref<'' | 'install' | 'release' | 'key'>('')
const keyID = ref('')
const keyPurpose = ref<'BTG_OWNED_SIGNING' | 'BTG_APPROVAL_SIGNING'>('BTG_OWNED_SIGNING')
const publicKey = ref('')
const allowedPluginIDs = ref('')
const pendingConfirmation = ref<Confirmation>()
const confirmationOpen = computed({ get: () => Boolean(pendingConfirmation.value), set: (open) => { if (!open) pendingConfirmation.value = undefined } })
let active = true
let generation = 0

const keyError = computed(() => publicKey.value && !/^[A-Za-z0-9_-]+$/.test(publicKey.value) ? t('admin.plugins.publicKeyInvalid') : undefined)
const rawAllowList = computed(() => allowedPluginIDs.value.split(/\n|,/).map((value) => value.trim()).filter(Boolean))
const allowList = computed(() => [...new Set(rawAllowList.value)])
const allowListError = computed(() => keyPurpose.value !== 'BTG_OWNED_SIGNING' ? undefined : !allowList.value.length ? t('admin.plugins.allowListRequired') : allowList.value.length !== rawAllowList.value.length ? t('admin.plugins.allowListDuplicate') : undefined)
const confirmationTitle = computed(() => pendingConfirmation.value?.kind === 'revoke' ? t('admin.plugins.confirmRevokeTitle') : t('admin.plugins.confirmDisableTitle'))
const confirmationDescription = computed(() => pendingConfirmation.value ? t(pendingConfirmation.value.kind === 'revoke' ? 'admin.plugins.confirmRevokeDescription' : 'admin.plugins.confirmDisableDescription', { name: pendingConfirmation.value.release.name, version: pendingConfirmation.value.release.version }) : '')
const confirmationAction = computed(() => pendingConfirmation.value?.kind === 'revoke' ? t('admin.plugins.confirmRevoke') : t('admin.plugins.confirmDisable'))

function releaseKey(release: Pick<PluginRelease, 'pluginId' | 'version'>) { return `${release.pluginId}:${release.version}` }
function validationLabel(value: string) { return value === 'VALIDATED_AT_REGISTRATION' ? t('admin.plugins.validated') : t('admin.plugins.signatureInvalid') }
function signatureLabel(value: string) { return value === 'VALID' ? t('admin.plugins.signatureValid') : value === 'INVALID' ? t('admin.plugins.signatureInvalid') : t('admin.plugins.signatureUnsigned') }
function trustLabel(release: PluginRelease) { return release.validationStatus === 'SIGNATURE_INVALID' ? t('admin.plugins.trustInvalid') : release.currentTrust === 'BTG_OWNED' ? t('admin.plugins.trustOwned') : release.currentTrust === 'BTG_APPROVED' ? t('admin.plugins.trustApproved') : t('admin.plugins.trustUnknown') }
function approvalLabel(value: string) { return value === 'ACTIVE' ? t('admin.plugins.approvalActive') : value === 'REVOKED' ? t('admin.plugins.approvalStateRevoked') : t('admin.plugins.approvalNone') }
function trustVariant(release: PluginRelease): 'neutral' | 'success' | 'warning' | 'danger' { return release.validationStatus === 'SIGNATURE_INVALID' ? 'danger' : release.currentTrust === 'BTG_OWNED' || release.currentTrust === 'BTG_APPROVED' ? 'success' : 'warning' }
function releaseMetadata(release: PluginRelease) { return [{ label: t('admin.plugins.digest'), value: release.artifactDigest }, { label: t('admin.plugins.validation'), value: validationLabel(release.validationStatus) }, { label: t('admin.plugins.signature'), value: signatureLabel(release.signatureStatus) }, { label: t('admin.plugins.approval'), value: approvalLabel(release.approvalState) }, { label: t('admin.plugins.enablement'), value: release.enabled ? t('admin.plugins.enabled') : t('admin.plugins.disabled') }, { label: t('admin.plugins.execution'), value: release.executionPermitted ? t('admin.plugins.yes') : t('admin.plugins.no') }, { label: t('admin.plugins.installedAt'), value: date(release.installedAt, 'dateTime') || release.installedAt }] }
function keyMetadata(key: PluginVerificationKey) { const items = [{ label: t('admin.plugins.purpose'), value: key.purpose === 'BTG_OWNED_SIGNING' ? t('admin.plugins.ownedSigning') : t('admin.plugins.approvalSigning') }, { label: t('admin.plugins.fingerprint'), value: key.fingerprint }]; if (key.allowedPluginIds.length) items.push({ label: t('admin.plugins.allowedPlugins'), value: key.allowedPluginIds.join(', ') }); return items }
function replace(release: PluginRelease) { plugins.value = plugins.value.map((value) => releaseKey(value) === releaseKey(release) ? release : value) }
function message(cause: unknown, fallback: 'install' | 'details' | 'lifecycle' | 'key' | 'keyUpdate') { const code = pluginManagementError(cause); return t(`admin.plugins.errors.${code === 'unavailable' ? fallback : code}`) }

async function load() { const request = ++generation; state.value = 'loading'; error.value = ''; try { const [releaseList, keyList] = await Promise.all([listPlugins(), listPluginKeys()]); if (!active || request !== generation) return; plugins.value = releaseList.plugins; keys.value = keyList.keys; state.value = 'ready'; announcement.value = t('admin.plugins.loadedAnnouncement') } catch (cause) { if (!active || request !== generation) return; state.value = isPluginManagementForbidden(cause) ? 'forbidden' : 'error' } }
function selectPackage(selected?: File) { packageFile.value = selected; error.value = ''; announcement.value = selected ? t('admin.plugins.selected', { filename: selected.name }) : '' }
async function install() { if (!packageFile.value || busy.value) return; const selected = packageFile.value; const request = ++generation; busy.value = 'install'; error.value = ''; try { const result = await installPlugin(selected); if (!active || request !== generation) return; packageFile.value = undefined; packageField.value?.clear(); announcement.value = t(result.replayed ? 'admin.plugins.replayed' : 'admin.plugins.installed', { name: result.release.name, version: result.release.version }); await reloadAuthoritative() } catch (cause) { if (active && request === generation) error.value = message(cause, 'install') } finally { if (active && request === generation) busy.value = '' } }
async function reloadAuthoritative() { const [releaseList, keyList] = await Promise.all([listPlugins(), listPluginKeys()]); if (!active) return; plugins.value = releaseList.plugins; keys.value = keyList.keys }
async function openDetail(release: PluginRelease, event: Event) { const open = (event.target as HTMLDetailsElement).open; const key = releaseKey(release); if (!open || details.value[key] || detailLoading.value) return; detailLoading.value = key; try { details.value = { ...details.value, [key]: await getPlugin(release) } } catch (cause) { error.value = message(cause, 'details') } finally { if (active) detailLoading.value = '' } }
async function lifecycle(release: PluginRelease, action: () => Promise<PluginRelease>, success: string) { if (busy.value) return; const request = ++generation; busy.value = 'release'; error.value = ''; try { const result = await action(); if (!active || request !== generation) return; replace(result); details.value = { ...details.value, [releaseKey(result)]: result }; announcement.value = success; await reloadAuthoritative() } catch (cause) { if (active && request === generation) { error.value = message(cause, 'lifecycle'); announcement.value = t('admin.plugins.lifecycleRejected'); try { await reloadAuthoritative() } catch { /* retain safe error */ } } } finally { if (active && request === generation) busy.value = '' } }
function approve(release: PluginRelease) { void lifecycle(release, () => approvePlugin(release), t('admin.plugins.approvalUpdated')) }
function setEnabled(release: PluginRelease, enabled: boolean) { void lifecycle(release, () => setPluginEnabled(release, enabled), t(enabled ? 'admin.plugins.pluginEnabled' : 'admin.plugins.pluginDisabled')) }
function requestConfirmation(kind: Confirmation['kind'], release: PluginRelease) { pendingConfirmation.value = { kind, release } }
function confirmLifecycle() { const pending = pendingConfirmation.value; if (!pending) return; pendingConfirmation.value = undefined; if (pending.kind === 'revoke') void lifecycle(pending.release, () => revokePluginApproval(pending.release), t('admin.plugins.approvalRevoked')); else setEnabled(pending.release, false) }
async function createKey() { if (busy.value || !keyID.value || !publicKey.value || keyError.value || allowListError.value) return; const request = ++generation; busy.value = 'key'; error.value = ''; try { await addPluginKey({ keyId: keyID.value.trim(), publicKey: publicKey.value.trim(), purpose: keyPurpose.value, allowedPluginIds: keyPurpose.value === 'BTG_OWNED_SIGNING' ? allowList.value : [] }); if (!active || request !== generation) return; keyID.value = ''; publicKey.value = ''; allowedPluginIDs.value = ''; announcement.value = t('admin.plugins.keyAdded'); await reloadAuthoritative(); await nextTick() } catch (cause) { if (active && request === generation) { error.value = message(cause, 'key'); announcement.value = t('admin.plugins.keyRejected'); try { await reloadAuthoritative() } catch { /* retain safe error */ } } } finally { if (active && request === generation) busy.value = '' } }
async function setKeyEnabled(key: PluginVerificationKey, enabled: boolean) { if (busy.value) return; const request = ++generation; busy.value = 'key'; error.value = ''; try { await setPluginKeyEnabled(key.keyId, enabled); if (!active || request !== generation) return; announcement.value = t(enabled ? 'admin.plugins.keyEnabled' : 'admin.plugins.keyDisabled'); await reloadAuthoritative() } catch (cause) { if (active && request === generation) { error.value = message(cause, 'keyUpdate'); announcement.value = t('admin.plugins.keyRejected'); try { await reloadAuthoritative() } catch { /* retain safe error */ } } } finally { if (active && request === generation) busy.value = '' } }

void load()
onBeforeUnmount(() => { active = false; generation += 1 })
</script>
