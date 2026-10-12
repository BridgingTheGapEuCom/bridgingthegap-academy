<template>
  <BtgPageContainer as="section" class="community-page" aria-labelledby="moderation-title">
    <RouterLink :to="communityPath(course())" class="community-page__back"><BtgIcon name="arrowLeft" decorative size="small" />{{ t('community.navigation.backToDiscussions') }}</RouterLink>
    <BtgPanel v-if="state.kind === 'loading'" role="status"><BtgPageTitle id="moderation-title" :title="t('community.moderation.loading')" /></BtgPanel>
    <BtgPanel v-else-if="state.kind === 'unavailable'" class="community-page__state" role="alert">
      <BtgPageTitle id="moderation-title" :title="t('community.moderation.unavailableTitle')" :description="t('community.moderation.unavailable')" />
      <BtgButton variant="secondary" @click="load()">{{ t('common.actions.tryAgain') }}</BtgButton>
    </BtgPanel>
    <template v-else>
      <BtgPageTitle id="moderation-title" focusable :title="t('community.moderation.title')" :description="t('community.moderation.helper')" />
      <BtgPanel v-if="state.page.threads.length === 0" variant="subtle"><p>{{ t('community.moderation.empty') }}</p></BtgPanel>
      <ol v-else class="community-threads">
        <li v-for="thread in state.page.threads" :key="thread.threadId">
          <BtgPanel as="article" padding="compact" class="community-thread-card">
            <div class="community-thread-card__heading">
              <RouterLink :to="communityModeratorThreadPath(course(), thread.threadId)" class="community-thread-card__title">{{ thread.title }}</RouterLink>
              <BtgBadge :variant="thread.state === 'HIDDEN' ? 'warning' : 'success'">{{ moderationStatus(thread.state) }}</BtgBadge>
            </div>
            <p class="community-thread-card__meta"><span>{{ t('community.threads.participant') }}</span><span>{{ t('community.counts.post', thread.postCount) }}</span><time :datetime="thread.updatedAt">{{ t('community.threads.updated', { date: date(thread.updatedAt, 'dateTime') }) }}</time></p>
          </BtgPanel>
        </li>
      </ol>
      <CommunityPagination :limit="state.page.limit" :offset="state.page.offset" :total="state.page.total" :copy="paginationCopy" @previous="changePage(-1)" @next="changePage(1)" />
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAuth } from '../auth/auth'
import BtgBadge from '../components/BtgBadge.vue'
import BtgButton from '../components/BtgButton.vue'
import BtgIcon from '../components/BtgIcon.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import CommunityPagination from '../components/CommunityPagination.vue'
import { communityPath, communityModeratorThreadPath, listCommunityThreadsForModeration, type CommunityModeratorThreadPage } from '../courses/community'
import { useApplicationFormatters } from '../i18n/formatters'

type State = { kind: 'loading' } | { kind: 'unavailable' } | { kind: 'ready'; page: CommunityModeratorThreadPage }
const route = useRoute()
const auth = useAuth()
const { t } = useI18n()
const { date } = useApplicationFormatters()
const state = ref<State>({ kind: 'loading' })
let active = true
let generation = 0
const paginationCopy = computed(() => ({ label: t('community.pagination.moderation'), range: (from: number, to: number, total: number) => t('community.pagination.range', { from, to, total }), previous: t('community.pagination.previous'), next: t('community.pagination.next') }))
const course = () => typeof route.params.courseId === 'string' ? route.params.courseId : ''

watch(() => route.params.courseId, () => { void load() }, { immediate: true })
onBeforeUnmount(() => { active = false; generation += 1 })
async function load(offset = 0) {
  const courseId = course()
  const request = ++generation
  state.value = { kind: 'loading' }
  try {
    const page = await listCommunityThreadsForModeration(courseId, offset, auth)
    if (active && request === generation && courseId === course()) state.value = { kind: 'ready', page }
  } catch {
    if (active && request === generation && courseId === course()) state.value = { kind: 'unavailable' }
  }
}
function changePage(delta: number) { if (state.value.kind === 'ready') void load(Math.max(0, state.value.page.offset + delta * state.value.page.limit)) }
function moderationStatus(value: 'VISIBLE' | 'HIDDEN') { return value === 'HIDDEN' ? t('community.moderation.statusHidden') : t('community.moderation.statusVisible') }
</script>
