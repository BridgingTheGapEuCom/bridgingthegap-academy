<template>
  <BtgPageContainer as="section" class="translation-list-page" aria-labelledby="translation-list-title">
    <BtgPanel v-if="state.kind === 'loading'" variant="subtle" role="status">
      <BtgPageTitle id="translation-list-title" :title="t('authoring.translations.listTitle')" :description="t('authoring.translations.loadingList')" />
    </BtgPanel>
    <BtgPanel v-else-if="state.kind === 'error'" variant="subtle" role="alert">
      <BtgPageTitle id="translation-list-title" :title="t('authoring.translations.listUnavailableTitle')" :description="t('authoring.translations.listUnavailableDescription')" />
      <BtgButton variant="secondary" @click="load">{{ t('authoring.translations.tryAgain') }}</BtgButton>
    </BtgPanel>
    <template v-else>
      <RouterLink class="btg-button btg-button--tertiary translation-list-page__back" to="/authoring">
        <BtgIcon name="arrowLeft" decorative />{{ t('authoring.translations.backToAuthoring') }}
      </RouterLink>
      <p class="btg-page-eyebrow">{{ t('authoring.translations.context') }}</p>
      <BtgPageTitle id="translation-list-title" :title="t('authoring.translations.listTitle')" :description="t('authoring.translations.listDescription')" />
      <BtgMetadataStrip class="translation-list-page__metadata" layout="three-column" :items="sourceMetadata" />
      <BtgPanel variant="subtle" padding="compact"><p>{{ t('authoring.translations.appLocaleIndependent') }}</p></BtgPanel>

      <BtgSection heading-id="translation-create-title">
        <template #heading>{{ t('authoring.translations.createTitle') }}</template>
        <p>{{ t('authoring.translations.createDescription', { version }) }}</p>
        <BtgPanel>
          <form class="translation-create" @submit.prevent="create">
            <BtgFormField
              :label="t('authoring.translations.targetLanguage')"
              :description="t('authoring.translations.targetLanguageHelp')"
              :error="languageError"
              :required-label="t('authoring.translations.required')"
              required
              v-slot="{ controlId, describedBy, invalid }"
            >
              <LanguagePicker :id="controlId" v-model="language" :labels="languagePickerLabels" :described-by="describedBy" :invalid="invalid" :disabled="creating" />
            </BtgFormField>
            <BtgButton type="submit" leading-icon="add" :loading="creating" :disabled="creating">
              {{ creating ? t('authoring.translations.creating') : t('authoring.translations.createAction') }}
            </BtgButton>
          </form>
          <p v-if="message" class="translation-list-page__message" role="alert">{{ message }}</p>
        </BtgPanel>
      </BtgSection>

      <BtgSection heading-id="translation-existing-title">
        <template #heading>{{ t('authoring.translations.existingTitle') }}</template>
        <p>{{ t('authoring.translations.existingDescription', { version }) }}</p>
        <BtgPanel v-if="!state.items.length" class="translation-list-page__empty" variant="subtle" role="status">
          <BtgIcon name="document" size="large" decorative />
          <div><h3>{{ t('authoring.translations.emptyTitle') }}</h3><p>{{ t('authoring.translations.emptyDescription') }}</p></div>
        </BtgPanel>
        <ol v-else class="translation-list" :aria-label="t('authoring.translations.translationListLabel', { version })">
          <li v-for="item in state.items" :key="item.translationId">
            <BtgPanel as="article" class="translation-list__item">
              <header><h3>{{ displayLanguage(item.targetLanguage) }}</h3><BtgBadge :variant="item.lifecycle === 'PUBLISHED' ? 'success' : 'neutral'">{{ lifecycleLabel(item.lifecycle) }}</BtgBadge></header>
              <BtgMetadataStrip :items="translationMetadata(item)" />
              <RouterLink class="btg-button btg-button--secondary" :to="`/authoring/translations/${item.translationId}`">
                <BtgIcon name="edit" decorative />{{ t('authoring.translations.openTranslation', { language: displayLanguage(item.targetLanguage) }) }}
              </RouterLink>
            </BtgPanel>
          </li>
        </ol>
      </BtgSection>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import BtgBadge from '../components/BtgBadge.vue'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgIcon from '../components/BtgIcon.vue'
import BtgMetadataStrip from '../components/BtgMetadataStrip.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import BtgSection from '../components/BtgSection.vue'
import LanguagePicker from '../components/LanguagePicker.vue'
import { getPublishedCourseVersionByID, type PublishedCourseVersionDetail } from '../courses/courses'
import { languageLabel } from '../i18n/languages'
import { createTranslation, listTranslations, type TranslationSummary } from '../translations/translations'

type ReadyState = { kind: 'ready'; source: PublishedCourseVersionDetail; items: TranslationSummary[] }
type State = { kind: 'loading' } | { kind: 'error' } | ReadyState
const route = useRoute()
const router = useRouter()
const { locale, t } = useI18n()
const state = ref<State>({ kind: 'loading' })
const language = ref('')
const languageError = ref('')
const message = ref('')
const creating = ref(false)
let generation = 0
let active = true

const courseId = computed(() => String(route.params.courseId || ''))
const version = computed(() => String(route.params.version || ''))
const sourceMetadata = computed(() => state.value.kind === 'ready' ? [
  { label: t('authoring.translations.sourceCourse'), value: state.value.source.title, icon: 'document' as const },
  { label: t('authoring.translations.sourceVersion'), value: state.value.source.version },
  { label: t('authoring.translations.sourceLanguage'), value: displayLanguage(state.value.source.sourceLanguage) },
] : [])
const languagePickerLabels = computed(() => ({
  placeholder: t('authoring.languagePicker.placeholder'), close: t('authoring.languagePicker.close'), open: t('authoring.languagePicker.open'), options: t('authoring.languagePicker.options'), empty: t('authoring.languagePicker.empty'),
  useTag: (tag: string) => t('authoring.languagePicker.useTag', { tag }), available: (count: number) => t('authoring.languagePicker.available', { count }), selected: (selected: string) => t('authoring.languagePicker.selected', { language: selected }),
}))

watch(() => [route.params.courseId, route.params.version], load, { immediate: true })
onBeforeUnmount(() => { active = false; generation += 1 })

function displayLanguage(value: string) { return languageLabel(value, locale.value) }
function lifecycleLabel(value: TranslationSummary['lifecycle']) { return t(`authoring.translations.status.${value}`) }
function progress(summary: TranslationSummary) { const count = summary.completeness.translatedFields; return t('authoring.translations.fieldsProgress', { translated: count, total: summary.completeness.totalTranslatableFields }, count) }
function translationMetadata(summary: TranslationSummary) { return [
  { label: t('authoring.translations.targetLanguage'), value: displayLanguage(summary.targetLanguage) },
  { label: t('authoring.translations.progressLabel'), value: progress(summary) },
  { label: t('authoring.translations.revisionLabel'), value: String(summary.revision) },
] }

async function load() {
  const request = ++generation
  state.value = { kind: 'loading' }
  languageError.value = ''
  message.value = ''
  try {
    const [translations, source] = await Promise.all([listTranslations(courseId.value, version.value), getPublishedCourseVersionByID(courseId.value, version.value)])
    if (!active || request !== generation) return
    state.value = { kind: 'ready', source, items: translations.translations }
  } catch {
    if (active && request === generation) state.value = { kind: 'error' }
  }
}

async function create() {
  if (creating.value || state.value.kind !== 'ready') return
  languageError.value = ''
  message.value = ''
  if (!language.value) languageError.value = t('authoring.translations.invalidLanguage')
  else if (language.value === state.value.source.sourceLanguage) languageError.value = t('authoring.translations.sameLanguage')
  else if (state.value.items.some((item) => item.targetLanguage === language.value)) languageError.value = t('authoring.translations.duplicateLanguage')
  if (languageError.value) return
  creating.value = true
  try {
    const item = await createTranslation(courseId.value, version.value, language.value)
    if (active) await router.push(`/authoring/translations/${item.translationId}`)
  } catch (error) {
    if (!active) return
    message.value = error instanceof APIProblemError && error.problem && 'code' in error.problem && error.problem.code === 'translation_already_exists'
      ? t('authoring.translations.duplicateTranslation') : t('authoring.translations.createUnavailable')
  } finally {
    if (active) creating.value = false
  }
}
</script>
