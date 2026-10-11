<template>
  <BtgPageContainer as="section" class="courses-page" aria-labelledby="courses-title">
    <BtgPanel v-if="state.kind === 'loading'" class="courses-page__state" variant="subtle" role="status" aria-live="polite">
      <BtgPageTitle id="courses-title" :title="t('courses.catalog.title')" :description="t('courses.catalog.loading')" />
    </BtgPanel>

    <BtgPanel v-else-if="state.kind === 'unavailable'" class="courses-page__state" variant="subtle" role="alert">
      <BtgPageTitle id="courses-title" :title="t('courses.catalog.unavailableTitle')" :description="t('courses.catalog.unavailableDescription')" />
      <BtgButton variant="secondary" @click="load()">{{ t('common.actions.tryAgain') }}</BtgButton>
    </BtgPanel>

    <div v-else class="courses-page__content">
      <p class="courses-page__eyebrow">{{ t('courses.catalog.eyebrow') }}</p>
      <BtgPageTitle id="courses-title" :title="t('courses.catalog.title')" :description="t('courses.catalog.description')" />

      <BtgPanel v-if="state.page.items.length === 0 && state.page.total === 0" variant="subtle" role="status">{{ t('courses.catalog.empty') }}</BtgPanel>
      <template v-else>
        <PublishedCourseCatalogList v-if="state.page.items.length" :courses="state.page.items" :copy="catalogCopy" :format-date="formatDate" />
        <BtgPanel v-else variant="subtle" role="status">{{ t('courses.catalog.emptyPage') }}</BtgPanel>
        <PublishedCourseCatalogPagination
          :limit="state.page.limit"
          :offset="state.page.offset"
          :total="state.page.total"
          :pending="paginationPending"
          :copy="paginationCopy"
          @previous="goToPreviousPage"
          @next="goToNextPage"
        />
      </template>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import PublishedCourseCatalogList from '../components/PublishedCourseCatalogList.vue'
import PublishedCourseCatalogPagination from '../components/PublishedCourseCatalogPagination.vue'
import { catalogOffsetFromRoute, listPublishedCourseCatalog, publishedCatalogPageSize, type PublishedCourseCatalogPage } from '../courses/courses'
import { useApplicationFormatters } from '../i18n/formatters'

type State =
  | { kind: 'loading' }
  | { kind: 'ready'; page: PublishedCourseCatalogPage }
  | { kind: 'unavailable' }

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const { date, number } = useApplicationFormatters()
const formatDate = (value: string) => date(value, 'date')
const catalogCopy = computed(() => ({
  listLabel: t('courses.catalog.listLabel'), version: t('courses.catalog.version'), language: t('courses.catalog.language'),
  published: t('courses.catalog.published'), license: t('courses.catalog.license'), contributors: t('courses.catalog.contributors'),
}))
const paginationCopy = computed(() => ({
  label: t('courses.catalog.paginationLabel'),
  range: (start: number, end: number, total: number) => t('courses.catalog.range', { start: number(start, 'count'), end: number(end, 'count'), total: number(total, 'count') }),
  previous: t('courses.catalog.previous'), next: t('courses.catalog.next'),
}))
const state = ref<State>({ kind: 'loading' })
const paginationPending = ref(false)
let requestVersion = 0
let active = true

watch(() => route.query.offset, (value) => {
  const offset = catalogOffsetFromRoute(value)
  if (offset === undefined) {
    void router.replace({ query: { ...route.query, offset: undefined } })
    return
  }
  void load(offset)
}, { immediate: true })

onBeforeUnmount(() => { active = false })

async function load(offset = catalogOffsetFromRoute(route.query.offset) ?? 0) {
  const version = ++requestVersion
  state.value = { kind: 'loading' }
  try {
    const page = await listPublishedCourseCatalog({ limit: publishedCatalogPageSize, offset })
    if (!active || version !== requestVersion) return
    if (page.items.length === 0 && offset > 0) {
      const normalizedOffset = page.total === 0 ? 0 : Math.floor((page.total - 1) / page.limit) * page.limit
      if (normalizedOffset !== offset) {
        await replaceOffset(normalizedOffset)
        return
      }
    }
    state.value = { kind: 'ready', page }
  } catch {
    if (!active || version !== requestVersion) return
    state.value = { kind: 'unavailable' }
  }
}

function goToPreviousPage() {
  if (state.value.kind !== 'ready' || paginationPending.value) return
  const offset = Math.max(0, state.value.page.offset - state.value.page.limit)
  paginationPending.value = true
  void setOffset(offset)
}

function goToNextPage() {
  if (state.value.kind !== 'ready' || paginationPending.value || state.value.page.offset + state.value.page.limit >= state.value.page.total) return
  paginationPending.value = true
  void setOffset(state.value.page.offset + state.value.page.limit)
}

async function setOffset(offset: number) {
  const query = { ...route.query, offset: offset === 0 ? undefined : String(offset) }
  try {
    await router.push({ query })
  } finally {
    paginationPending.value = false
  }
}

async function replaceOffset(offset: number) {
  const query = { ...route.query, offset: offset === 0 ? undefined : String(offset) }
  await router.replace({ query })
}
</script>
