<template>
  <BtgPageContainer as="section" class="plugin-management" width="application" aria-labelledby="plugin-management-title">
    <p class="sr-only" role="status" aria-live="polite">{{ announcement }}</p>
    <header class="plugin-management__header">
      <p class="plugin-management__eyebrow">Administration</p>
      <h1 id="plugin-management-title" ref="heading" tabindex="-1">Plugin management</h1>
      <p>Install immutable plugin packages and manage their current trust, approval, enablement, and public verification keys.</p>
    </header>

    <section v-if="state === 'loading'" role="status"><p>Loading plugin management…</p></section>
    <section v-else-if="state === 'forbidden'" aria-labelledby="plugin-management-forbidden"><h2 id="plugin-management-forbidden">Access denied</h2><p>Your account does not have permission to manage plugins.</p></section>
    <section v-else-if="state === 'error'" aria-labelledby="plugin-management-error"><h2 id="plugin-management-error">Plugin management unavailable</h2><p>We couldn’t load plugin management right now.</p><BtgButton @click="load">Try again</BtgButton></section>

    <template v-else>
      <p v-if="error" class="plugin-management__error" role="alert">{{ error }}</p>

      <section class="plugin-management__section" aria-labelledby="plugin-install-title">
        <h2 id="plugin-install-title">Install plugin package</h2>
        <p>A package is validated by the server before registration. Installation does not execute plugin code.</p>
        <form @submit.prevent="install">
          <BtgFormField label="Plugin ZIP package" required v-slot="{ controlId, describedBy, invalid }">
            <input :id="controlId" ref="packageInput" type="file" accept=".zip,application/zip" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" :disabled="Boolean(busy)" @change="selectPackage">
          </BtgFormField>
          <p v-if="packageFile" class="plugin-management__selection"><strong>{{ packageFile.name }}</strong> · {{ formatBytes(packageFile.size) }}</p>
          <div class="plugin-management__actions"><BtgButton type="submit" :disabled="Boolean(busy) || !packageFile">{{ busy === 'install' ? 'Installing plugin…' : 'Install plugin' }}</BtgButton></div>
        </form>
      </section>

      <section class="plugin-management__section" aria-labelledby="plugin-releases-title">
        <h2 id="plugin-releases-title">Installed releases</h2>
        <p>Validation, trust, approval, enablement, and execution eligibility are independent states.</p>
        <p v-if="!plugins.length">No plugin releases are installed.</p>
        <ol v-else class="plugin-management__list" aria-label="Installed plugin releases">
          <li v-for="release in plugins" :key="releaseKey(release)">
            <article class="plugin-management__card">
              <header><h3>{{ release.name }} <span class="plugin-management__version">{{ release.version }}</span></h3><p>{{ release.pluginId }}</p></header>
              <dl class="plugin-management__facts">
                <div><dt>Digest</dt><dd><code :title="release.artifactDigest">{{ abbreviated(release.artifactDigest) }}</code></dd></div>
                <div><dt>Validation</dt><dd>{{ validationLabel(release.validationStatus) }}</dd></div>
                <div><dt>Signature</dt><dd>{{ release.signatureStatus }}</dd></div>
                <div><dt>Trust</dt><dd>{{ trustLabel(release) }}</dd></div>
                <div><dt>Approval</dt><dd>{{ release.approvalState }}</dd></div>
                <div><dt>Stored enablement</dt><dd>{{ release.enabled ? 'Enabled' : 'Disabled' }}</dd></div>
                <div><dt>Execution eligible now</dt><dd>{{ release.executionPermitted ? 'Yes' : 'No' }}</dd></div>
                <div><dt>Installed</dt><dd>{{ formatDate(release.installedAt) }}</dd></div>
              </dl>
              <p v-if="release.validationStatus === 'SIGNATURE_INVALID'" class="plugin-management__warning">The recognized signature is invalid. This is not an UNKNOWN trust state.</p>
              <p v-else-if="release.currentTrust === 'UNKNOWN'" class="plugin-management__note">This is a valid installed release that is not currently BTG-owned or BTG-approved.</p>
              <p>Widgets: {{ release.entrypoints.length ? release.entrypoints.map((entrypoint) => `${entrypoint.name} (${entrypoint.type})`).join(', ') : 'None' }}</p>
              <details @toggle="openDetail(release, $event)"><summary>Release details</summary>
                <p v-if="detailLoading === releaseKey(release)" role="status">Loading release details…</p>
                <div v-else-if="details[releaseKey(release)]" class="plugin-management__details">
                  <p><strong>Publisher:</strong> {{ details[releaseKey(release)]!.publisherName }}</p>
                  <p v-if="details[releaseKey(release)]!.homepage"><a :href="details[releaseKey(release)]!.homepage" rel="noreferrer">Plugin homepage</a></p>
                  <p>{{ details[releaseKey(release)]!.description }}</p>
                  <ul><li v-for="entrypoint in details[releaseKey(release)]!.entrypoints" :key="entrypoint.id"><strong>{{ entrypoint.name }}</strong> · {{ entrypoint.id }} · {{ entrypoint.type }}</li></ul>
                </div>
              </details>
              <div class="plugin-management__actions" aria-label="Release lifecycle actions">
                <BtgButton v-if="release.approvalState !== 'ACTIVE'" type="button" variant="secondary" :disabled="Boolean(busy) || release.validationStatus === 'SIGNATURE_INVALID'" @click="approve(release)">Approve</BtgButton>
                <BtgButton v-else type="button" variant="secondary" :disabled="Boolean(busy)" @click="revoke(release)">Revoke approval</BtgButton>
                <BtgButton v-if="!release.enabled" type="button" :disabled="Boolean(busy) || !release.executionPermitted" @click="setEnabled(release, true)">Enable</BtgButton>
                <BtgButton v-else type="button" variant="secondary" :disabled="Boolean(busy)" @click="setEnabled(release, false)">Disable</BtgButton>
              </div>
              <p v-if="release.enabled || release.approvalState === 'ACTIVE'" class="plugin-management__note">Disabling or revoking approval keeps published Course content and Dashboard placements. Affected widgets may become unavailable.</p>
            </article>
          </li>
        </ol>
      </section>

      <section class="plugin-management__section" aria-labelledby="plugin-keys-title">
        <h2 id="plugin-keys-title">Verification keys</h2>
        <p>Only public Ed25519 verification key material belongs here. Never paste a private seed or signing key: 32-byte seeds cannot be reliably distinguished from public-key bytes by length alone.</p>
        <form class="plugin-management__key-form" @submit.prevent="createKey">
          <BtgFormField label="Key ID" required v-slot="{ controlId, describedBy, invalid }"><input :id="controlId" v-model.trim="keyID" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" :disabled="Boolean(busy)"></BtgFormField>
          <BtgFormField label="Key purpose" required v-slot="{ controlId, describedBy, invalid }"><select :id="controlId" v-model="keyPurpose" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" :disabled="Boolean(busy)"><option value="BTG_OWNED_SIGNING">BTG owned signing</option><option value="BTG_APPROVAL_SIGNING">BTG approval signing</option></select></BtgFormField>
          <BtgFormField label="Public key (unpadded base64url)" required :error="keyError" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" v-model.trim="publicKey" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" :disabled="Boolean(busy)" spellcheck="false" /></BtgFormField>
          <BtgFormField v-if="keyPurpose === 'BTG_OWNED_SIGNING'" label="Allowed plugin IDs" description="One reverse-domain plugin ID per line. This key is not trusted for every plugin." required :error="allowListError" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" v-model="allowedPluginIDs" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" :disabled="Boolean(busy)" /></BtgFormField>
          <p v-else class="plugin-management__note">Approval signing keys cannot have a plugin allow-list.</p>
          <div class="plugin-management__actions"><BtgButton type="submit" :disabled="Boolean(busy) || !!keyError || !!allowListError">{{ busy === 'key' ? 'Adding key…' : 'Add verification key' }}</BtgButton></div>
        </form>
        <p v-if="!keys.length">No verification keys are registered.</p>
        <ul v-else class="plugin-management__list" aria-label="Plugin verification keys"><li v-for="key in keys" :key="key.keyId"><article class="plugin-management__card"><h3>{{ key.keyId }}</h3><dl class="plugin-management__facts"><div><dt>Purpose</dt><dd>{{ key.purpose }}</dd></div><div><dt>Fingerprint</dt><dd><code>{{ key.fingerprint }}</code></dd></div><div><dt>State</dt><dd>{{ key.enabled ? 'Enabled' : 'Disabled' }}</dd></div><div v-if="key.allowedPluginIds.length"><dt>Allowed plugins</dt><dd>{{ key.allowedPluginIds.join(', ') }}</dd></div></dl><div class="plugin-management__actions"><BtgButton v-if="!key.enabled" :disabled="Boolean(busy)" @click="setKeyEnabled(key, true)">Enable key</BtgButton><BtgButton v-else variant="secondary" :disabled="Boolean(busy)" @click="setKeyEnabled(key, false)">Disable key</BtgButton></div><p class="plugin-management__note">Disabling a key can immediately change trust and execution eligibility for affected releases.</p></article></li></ul>
      </section>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { APIProblemError } from '../api/client'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import { addPluginKey, approvePlugin, getPlugin, installPlugin, isPluginManagementForbidden, listPluginKeys, listPlugins, pluginManagementMessage, revokePluginApproval, setPluginEnabled, setPluginKeyEnabled, type PluginRelease, type PluginVerificationKey } from '../plugins/management'

type PageState = 'loading' | 'ready' | 'forbidden' | 'error'
const state = ref<PageState>('loading'); const plugins = ref<PluginRelease[]>([]); const keys = ref<PluginVerificationKey[]>([])
const details = ref<Record<string, PluginRelease>>({}); const detailLoading = ref(''); const packageFile = ref<File>(); const packageInput = ref<HTMLInputElement>(); const heading = ref<HTMLElement>(); const announcement = ref(''); const error = ref(''); const busy = ref<'' | 'install' | 'release' | 'key'>('')
const keyID = ref(''); const keyPurpose = ref<'BTG_OWNED_SIGNING' | 'BTG_APPROVAL_SIGNING'>('BTG_OWNED_SIGNING'); const publicKey = ref(''); const allowedPluginIDs = ref('')
let active = true; let generation = 0
const keyError = computed(() => publicKey.value && !/^[A-Za-z0-9_-]+$/.test(publicKey.value) ? 'Use unpadded base64url public key bytes.' : undefined)
const rawAllowList = computed(() => allowedPluginIDs.value.split(/\n|,/).map((value) => value.trim()).filter(Boolean))
const allowList = computed(() => [...new Set(rawAllowList.value)])
const allowListError = computed(() => {
  if (keyPurpose.value !== 'BTG_OWNED_SIGNING') return undefined
  if (!allowList.value.length) return 'Add at least one allowed plugin ID.'
  if (allowList.value.length !== rawAllowList.value.length) return 'Remove duplicate plugin IDs from the allow-list.'
  return undefined
})
function releaseKey(release: Pick<PluginRelease, 'pluginId' | 'version'>) { return `${release.pluginId}:${release.version}` }
function abbreviated(value: string) { return value.length > 18 ? `${value.slice(0, 12)}…${value.slice(-6)}` : value }
function formatDate(value: string) { const date = new Date(value); return Number.isNaN(date.valueOf()) ? value : date.toLocaleString() }
function formatBytes(value: number) { return value < 1024 ? `${value} bytes` : `${(value / 1024 / 1024).toFixed(1)} MiB` }
function validationLabel(value: string) { return value === 'VALIDATED_AT_REGISTRATION' ? 'Validated at registration' : 'Signature invalid' }
function trustLabel(release: PluginRelease) { return release.currentTrust ?? 'Not classified because the signature is invalid' }
function replace(release: PluginRelease) { plugins.value = plugins.value.map((value) => releaseKey(value) === releaseKey(release) ? release : value) }
async function load() {
  const request = ++generation; state.value = 'loading'; error.value = ''
  try { const [releaseList, keyList] = await Promise.all([listPlugins(), listPluginKeys()]); if (!active || request !== generation) return; plugins.value = releaseList.plugins; keys.value = keyList.keys; state.value = 'ready'; announcement.value = 'Plugin management loaded.' }
  catch (cause) { if (!active || request !== generation) return; state.value = isPluginManagementForbidden(cause) ? 'forbidden' : 'error' }
}
function selectPackage(event: Event) { packageFile.value = (event.target as HTMLInputElement).files?.[0]; error.value = ''; announcement.value = packageFile.value ? `Selected ${packageFile.value.name}.` : '' }
async function install() {
  if (!packageFile.value || busy.value) return; const selected = packageFile.value; const request = ++generation; busy.value = 'install'; error.value = ''
  try { const result = await installPlugin(selected); if (!active || request !== generation) return; packageFile.value = undefined; if (packageInput.value) packageInput.value.value = ''; announcement.value = result.replayed ? `Plugin ${result.release.name} ${result.release.version} was already registered.` : `Plugin ${result.release.name} ${result.release.version} installed.`; await reloadAuthoritative() }
  catch (cause) { if (active && request === generation) error.value = pluginManagementMessage(cause, 'The plugin package could not be installed.') }
  finally { if (active && request === generation) busy.value = '' }
}
async function reloadAuthoritative() { const [releaseList, keyList] = await Promise.all([listPlugins(), listPluginKeys()]); if (!active) return; plugins.value = releaseList.plugins; keys.value = keyList.keys }
async function openDetail(release: PluginRelease, event: Event) { const open = (event.target as HTMLDetailsElement).open; const key = releaseKey(release); if (!open || details.value[key] || detailLoading.value) return; detailLoading.value = key; try { details.value = { ...details.value, [key]: await getPlugin(release) } } catch { error.value = 'Release details could not be loaded.' } finally { if (active) detailLoading.value = '' } }
async function lifecycle(release: PluginRelease, action: () => Promise<PluginRelease>, success: string) { if (busy.value) return; const request = ++generation; busy.value = 'release'; error.value = ''; try { const result = await action(); if (!active || request !== generation) return; replace(result); details.value = { ...details.value, [releaseKey(result)]: result }; announcement.value = success; await reloadAuthoritative() } catch (cause) { if (active && request === generation) { error.value = pluginManagementMessage(cause, 'The plugin lifecycle change could not be completed.'); announcement.value = 'Plugin state changed or the operation was rejected. Current state has been reloaded.'; try { await reloadAuthoritative() } catch { /* retain the safe operation error */ } } } finally { if (active && request === generation) busy.value = '' } }
function approve(release: PluginRelease) { void lifecycle(release, () => approvePlugin(release), 'Plugin approval updated.') }
function revoke(release: PluginRelease) { if (confirm('Revoke this exact release approval? It may make affected widgets unavailable, but does not remove Course content or Dashboard placements.')) void lifecycle(release, () => revokePluginApproval(release), 'Plugin approval revoked.') }
function setEnabled(release: PluginRelease, enabled: boolean) { if (!enabled || confirm('Disable this release? It remains installed; affected widgets may become unavailable.')) void lifecycle(release, () => setPluginEnabled(release, enabled), enabled ? 'Plugin enabled.' : 'Plugin disabled.') }
async function createKey() { if (busy.value || !keyID.value || !publicKey.value || keyError.value || allowListError.value) return; const request = ++generation; busy.value = 'key'; error.value = ''; try { await addPluginKey({ keyId: keyID.value, publicKey: publicKey.value, purpose: keyPurpose.value, allowedPluginIds: keyPurpose.value === 'BTG_OWNED_SIGNING' ? allowList.value : [] }); if (!active || request !== generation) return; keyID.value = ''; publicKey.value = ''; allowedPluginIDs.value = ''; announcement.value = 'Verification key added.'; await reloadAuthoritative(); await nextTick(); heading.value?.focus() } catch (cause) { if (active && request === generation) { error.value = pluginManagementMessage(cause, 'The verification key could not be added.'); announcement.value = 'Verification-key state changed or the operation was rejected. Current state has been reloaded.'; try { await reloadAuthoritative() } catch { /* retain the safe operation error */ } } } finally { if (active && request === generation) busy.value = '' } }
async function setKeyEnabled(key: PluginVerificationKey, enabled: boolean) { if (busy.value) return; const request = ++generation; busy.value = 'key'; error.value = ''; try { await setPluginKeyEnabled(key.keyId, enabled); if (!active || request !== generation) return; announcement.value = enabled ? 'Verification key enabled.' : 'Verification key disabled.'; await reloadAuthoritative() } catch (cause) { if (active && request === generation) { error.value = pluginManagementMessage(cause, 'The verification key could not be updated.'); announcement.value = 'Verification-key state changed or the operation was rejected. Current state has been reloaded.'; try { await reloadAuthoritative() } catch { /* retain the safe operation error */ } } } finally { if (active && request === generation) busy.value = '' } }
void load(); onBeforeUnmount(() => { active = false; generation += 1 })
</script>

<style scoped>
.plugin-management { overflow-wrap: anywhere; }
.plugin-management__header, .plugin-management__section { max-width: 78rem; margin-block-end: 2rem; }
.plugin-management__eyebrow { font-weight: 700; text-transform: uppercase; letter-spacing: .08em; }
.plugin-management__section { border-block-start: 1px solid var(--color-border, #b8b8b8); padding-block-start: 1.25rem; }
.plugin-management__list { display: grid; gap: 1rem; padding: 0; list-style: none; }
.plugin-management__card { border: 1px solid var(--color-border, #b8b8b8); border-radius: .5rem; padding: 1rem; overflow-wrap: anywhere; }
.plugin-management__facts { display: grid; grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr)); gap: .75rem; }
.plugin-management__facts div { min-width: 0; }.plugin-management__facts dt { font-weight: 700; }.plugin-management__facts dd { margin: .25rem 0 0; }
.plugin-management__actions { display: flex; flex-wrap: wrap; gap: .5rem; margin-block-start: 1rem; }.plugin-management__key-form { display: grid; gap: 1rem; max-width: 48rem; }.plugin-management input, .plugin-management select, .plugin-management textarea { box-sizing: border-box; max-inline-size: 100%; width: 100%; }.plugin-management textarea { min-height: 7rem; font-family: ui-monospace, monospace; }.plugin-management__error, .plugin-management__warning { color: var(--color-danger, #9b1c1c); }.plugin-management__note { font-size: .95rem; }.plugin-management__version { font-size: .9em; font-weight: 400; }
@media (max-width: 20rem) { .plugin-management__actions > * { width: 100%; }.plugin-management__facts { grid-template-columns: 1fr; } }
</style>
