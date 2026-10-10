<template>
  <BtgPageContainer as="section" class="settings-page" aria-labelledby="settings-title">
    <div class="settings-page__layout">
      <aside class="settings-page__navigation" :aria-label="t('settings.navigationLabel')">
        <h2>{{ t('settings.title') }}</h2>
        <nav :aria-label="t('settings.navigationLabel')">
          <RouterLink to="/settings">{{ t('settings.preferences') }}</RouterLink>
        </nav>
      </aside>

      <div class="settings-page__content">
        <BtgPageTitle id="settings-title" :title="t('settings.preferences')" :description="t('settings.preferencesDescription')" />

        <BtgPanel v-if="preferences.state.value.loading && !stored" variant="subtle" role="status">
          <p>{{ t('settings.loading') }}</p>
        </BtgPanel>
        <BtgPanel v-else-if="!stored" variant="subtle" role="alert">
          <h2>{{ t('settings.loadErrorTitle') }}</h2>
          <p>{{ t('settings.loadErrorDescription') }}</p>
          <BtgButton variant="secondary" @click="reload">{{ t('common.actions.tryAgain') }}</BtgButton>
        </BtgPanel>

        <form v-else @submit.prevent="save">
          <BtgSection heading-id="settings-language-title">
            <template #heading>{{ t('settings.language') }}</template>
            <BtgFormField :label="t('settings.language')" :description="t('settings.languageDescription')">
              <template #default="{ controlId, describedBy }">
                <BtgSelect :id="controlId" v-model="draft.applicationLocale" :aria-describedby="describedBy" :disabled="saving">
                  <option value="">{{ t('settings.automaticLanguage') }}</option>
                  <option v-for="locale in productionLocales" :key="locale.id" :value="locale.id">{{ locale.nativeName }}</option>
                </BtgSelect>
              </template>
            </BtgFormField>
            <p class="settings-page__note">{{ t('settings.languageCourseNote') }}</p>
          </BtgSection>

          <BtgSection heading-id="settings-appearance-title">
            <template #heading>{{ t('settings.appearance') }}</template>
            <p>{{ t('settings.appearanceDescription') }}</p>
            <fieldset class="settings-page__appearance" :disabled="saving">
              <legend class="sr-only">{{ t('settings.appearance') }}</legend>
              <div v-for="option in appearanceOptions" :key="option.value" class="settings-page__appearance-option">
                <BtgRadio v-model="draft.appearance" name="appearance" :value="option.value" :label="option.label" :aria-describedby="`settings-appearance-${option.value}`" />
                <span :id="`settings-appearance-${option.value}`">{{ option.description }}</span>
              </div>
            </fieldset>
          </BtgSection>

          <BtgPanel v-if="notice" variant="subtle" role="status"><p>{{ notice }}</p></BtgPanel>
          <BtgPanel v-if="error" variant="subtle" role="alert">
            <p>{{ error }}</p>
            <BtgButton v-if="conflict" type="button" variant="secondary" @click="reload">{{ t('settings.reload') }}</BtgButton>
          </BtgPanel>

          <AuthoringDirtyActionBar
            :show="dirty"
            :busy="saving"
            :disabled="saving"
            :message="t('settings.dirty')"
            :discard-label="t('settings.discard')"
            :save-label="t('settings.save')"
            :busy-label="t('settings.saving')"
            :save-disabled="conflict"
            @discard="discard"
          />
        </form>
      </div>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { APIProblemError } from '../api/client'
import { useAuth } from '../auth/auth'
import AuthoringDirtyActionBar from '../components/AuthoringDirtyActionBar.vue'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import BtgRadio from '../components/BtgRadio.vue'
import BtgSection from '../components/BtgSection.vue'
import BtgSelect from '../components/BtgSelect.vue'
import { applicationLocales } from '../i18n/registry'
import { useUserPreferences, type UserPreferencesResponse } from '../preferences/preferences'
import type { StoredAppearance } from '../preferences/appearance'

const { t } = useI18n()
const auth = useAuth()
const preferences = useUserPreferences()
const stored = computed(() => preferences.state.value.preferences)
const productionLocales = computed(() => Object.values(applicationLocales).filter((locale) => locale.production))
const draft = reactive<{ applicationLocale: string; appearance: StoredAppearance }>({ applicationLocale: '', appearance: 'SYSTEM' })
const baseline = ref<UserPreferencesResponse>()
const saving = ref(false)
const conflict = ref(false)
const error = ref<string>()
const notice = ref<string>()
const dirty = computed(() => baseline.value !== undefined && (draft.applicationLocale !== (baseline.value.applicationLocale ?? '') || draft.appearance !== baseline.value.appearance))
const appearanceOptions = computed(() => [
  { value: 'SYSTEM' as const, label: t('settings.system'), description: t('settings.systemDescription') },
  { value: 'LIGHT' as const, label: t('settings.light'), description: t('settings.lightDescription') },
  { value: 'DARK' as const, label: t('settings.dark'), description: t('settings.darkDescription') },
])

watch(stored, (next) => {
  if (!next || dirty.value) return
  baseline.value = next
  draft.applicationLocale = next.applicationLocale ?? ''
  draft.appearance = next.appearance
  conflict.value = false
}, { immediate: true })

function discard() {
  if (!baseline.value) return
  draft.applicationLocale = baseline.value.applicationLocale ?? ''
  draft.appearance = baseline.value.appearance
  conflict.value = false
  error.value = undefined
  notice.value = undefined
}

async function reload() {
  error.value = undefined
  notice.value = undefined
  conflict.value = false
  await preferences.refresh()
  if (preferences.state.value.preferences) {
    baseline.value = preferences.state.value.preferences
    discard()
  }
}

async function save() {
  if (!baseline.value || saving.value || !dirty.value || conflict.value) return
  saving.value = true
  error.value = undefined
  notice.value = undefined
  try {
    const response = await auth.request<unknown>('/api/me/preferences', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        applicationLocale: draft.applicationLocale || null,
        appearance: draft.appearance,
        revision: baseline.value.revision,
      }),
    })
    if (!await preferences.applySaved(response)) {
      error.value = t('settings.saveError')
      return
    }
    const updated = preferences.state.value.preferences
    if (!updated) {
      error.value = t('settings.saveError')
      return
    }
    baseline.value = updated
    draft.applicationLocale = updated.applicationLocale ?? ''
    draft.appearance = updated.appearance
    notice.value = t('settings.saved')
  } catch (reason) {
    if (reason instanceof APIProblemError && reason.status === 409) {
      conflict.value = true
      error.value = t('settings.conflict')
    } else error.value = t('settings.saveError')
  } finally {
    saving.value = false
    await nextTick()
  }
}
</script>

<style scoped>
.settings-page { padding-block: var(--academy-page-padding-block); }
.settings-page__layout { display: grid; grid-template-columns: minmax(11rem, 15rem) minmax(0, 1fr); align-items: start; gap: clamp(var(--academy-space-6), 6vw, var(--academy-space-8)); }
.settings-page__navigation { display: grid; gap: var(--academy-space-3); color: var(--academy-text-primary); }
.settings-page__navigation h2 { margin: 0; font-size: var(--academy-font-size-subsection-title); }
.settings-page__navigation nav { display: grid; gap: var(--academy-space-1); }
.settings-page__navigation a { display: block; border-inline-start: 0.2rem solid transparent; color: var(--academy-text-secondary); padding: var(--academy-space-2) var(--academy-space-3); text-decoration: none; }
.settings-page__navigation a[aria-current="page"] { border-color: var(--academy-action-primary-bg); background: var(--academy-surface-selected); color: var(--academy-text-primary); font-weight: var(--academy-font-weight-strong); }
.settings-page__content, .settings-page__content form { display: grid; min-width: 0; gap: var(--academy-space-6); }
.settings-page__content form { max-inline-size: 48rem; }
.settings-page__note, .settings-page__appearance-option > span { margin: 0; color: var(--academy-text-secondary); }
.settings-page__appearance { display: grid; gap: var(--academy-space-3); border: 0; margin: 0; padding: 0; }
.settings-page__appearance-option { display: grid; gap: var(--academy-space-1); border: 1px solid var(--academy-border-default); background: var(--academy-surface-default); padding: var(--academy-space-4); }
.settings-page__appearance-option :deep(.btg-radio) { align-items: center; }
@media (max-width: 48rem) { .settings-page__layout { grid-template-columns: 1fr; gap: var(--academy-space-5); } .settings-page__navigation nav { grid-template-columns: minmax(0, 1fr); } }
</style>
