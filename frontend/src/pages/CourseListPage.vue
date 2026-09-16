<template>
  <BtgPageContainer as="section" class="courses-page" width="reading" aria-labelledby="courses-title">
    <div v-if="state.kind === 'loading'" class="courses-page__state" role="status" aria-live="polite">
      <h1 id="courses-title">Courses</h1>
      <p>Loading published courses…</p>
    </div>

    <div v-else-if="state.kind === 'unavailable'" class="courses-page__state" role="alert">
      <h1 id="courses-title">Courses unavailable</h1>
      <p>We couldn’t load published courses right now. Please try again.</p>
      <BtgButton variant="secondary" @click="load()">Try again</BtgButton>
    </div>

    <div v-else class="courses-page__content">
      <p class="courses-page__eyebrow">Bridging the Gap Academy</p>
      <h1 id="courses-title">Courses</h1>
      <p class="courses-page__intro">Structured learning resources for understanding integration and the systems around it.</p>

      <p v-if="state.page.items.length === 0 && state.page.total === 0" class="courses-page__empty" role="status">No published courses are available yet.</p>
      <template v-else>
        <PublishedCourseCatalogList v-if="state.page.items.length" :courses="state.page.items" />
        <p v-else class="courses-page__empty" role="status">No published courses are available on this page.</p>
        <PublishedCourseCatalogPagination
          :limit="state.page.limit"
          :offset="state.page.offset"
          :total="state.page.total"
          :pending="paginationPending"
          @previous="goToPreviousPage"
          @next="goToNextPage"
        />
      </template>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import PublishedCourseCatalogList from '../components/PublishedCourseCatalogList.vue'
import PublishedCourseCatalogPagination from '../components/PublishedCourseCatalogPagination.vue'
import { catalogOffsetFromRoute, listPublishedCourseCatalog, publishedCatalogPageSize, type PublishedCourseCatalogPage } from '../courses/courses'

type State =
  | { kind: 'loading' }
  | { kind: 'ready'; page: PublishedCourseCatalogPage }
  | { kind: 'unavailable' }

const route = useRoute()
const router = useRouter()
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
