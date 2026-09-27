<template>
  <BtgPageContainer as="section" aria-labelledby="translation-list-title">
    <RouterLink to="/authoring">Back to Authoring</RouterLink>
    <div v-if="state === 'loading'" role="status"><h1 id="translation-list-title">Loading translations</h1></div>
    <div v-else-if="state === 'error'" role="alert"><h1 id="translation-list-title">Translations unavailable</h1><p>We could not load translations for this source version.</p><BtgButton @click="load">Try again</BtgButton></div>
    <template v-else>
      <h1 id="translation-list-title">Translations</h1>
      <p>Translations of source version <strong>{{ version }}</strong>. Each workspace remains linked to this exact version.</p>
      <form class="translation-create" @submit.prevent="create">
        <BtgFormField label="Target language" description="Select the language for this translation." :error="languageError" v-slot="{ controlId, describedBy, invalid }">
          <LanguagePicker :id="controlId" v-model="language" :described-by="describedBy" :invalid="invalid" :disabled="creating" />
        </BtgFormField>
        <p v-if="message" role="status">{{ message }}</p>
        <BtgButton type="submit" :disabled="creating">{{ creating ? 'Creating…' : 'Create translation' }}</BtgButton>
      </form>
      <p v-if="items.length === 0" role="status">No translations have been created for this source version.</p>
      <ol v-else class="translation-list">
        <li v-for="item in items" :key="item.translationId">
          <article><h2>{{ languageLabel(item.targetLanguage) }}</h2><p>{{ item.completeness.translatedFields }} of {{ item.completeness.totalTranslatableFields }} fields translated · {{ item.lifecycle }}</p><RouterLink :to="`/authoring/translations/${item.translationId}`">Open {{ languageLabel(item.targetLanguage) }} translation</RouterLink></article>
        </li>
      </ol>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import LanguagePicker from '../components/LanguagePicker.vue'
import { languageLabel } from '../i18n/languages'
import { createTranslation, listTranslations, type TranslationSummary } from '../translations/translations'

const route = useRoute()
const router = useRouter()
const courseId = String(route.params.courseId || '')
const version = String(route.params.version || '')
const items = ref<TranslationSummary[]>([])
const state = ref<'loading' | 'ready' | 'error'>('loading')
const language = ref('')
const languageError = ref('')
const message = ref('')
const creating = ref(false)
let generation = 0
let active = true

watch(() => [route.params.courseId, route.params.version], load, { immediate: true })
onBeforeUnmount(() => { active = false })

async function load() {
  const request = ++generation
  state.value = 'loading'
  try {
    const result = await listTranslations(String(route.params.courseId || ''), String(route.params.version || ''))
    if (!active || request !== generation) return
    items.value = result.translations
    state.value = 'ready'
  } catch {
    if (active && request === generation) state.value = 'error'
  }
}

async function create() {
  languageError.value = ''
  if (!language.value || creating.value) {
    languageError.value = 'Enter or select a valid language tag.'
    return
  }
  creating.value = true
  message.value = ''
  try {
    const item = await createTranslation(courseId, version, language.value)
    if (!active) return
    await router.push(`/authoring/translations/${item.translationId}`)
  } catch (error) {
    message.value = error instanceof APIProblemError && error.problem && 'code' in error.problem && error.problem.code === 'translation_already_exists'
      ? 'A translation for that language already exists. Reload the list to open it.'
      : 'We could not create this translation. Your language choice is still here.'
  } finally {
    if (active) creating.value = false
  }
}
</script>
