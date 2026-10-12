<template>
  <BtgPageContainer as="section" class="community-page" aria-labelledby="moderator-thread-title">
    <RouterLink :to="communityModerationPath(course())" class="community-page__back"><BtgIcon name="arrowLeft" decorative size="small" />{{ t('community.navigation.backToModeration') }}</RouterLink>
    <BtgPanel v-if="state.kind === 'loading'" role="status"><BtgPageTitle id="moderator-thread-title" :title="t('community.moderation.loadingThread')" /></BtgPanel>
    <BtgPanel v-else-if="state.kind === 'unavailable'" class="community-page__state" role="alert">
      <BtgPageTitle id="moderator-thread-title" :title="t('community.moderation.threadUnavailableTitle')" :description="t('community.moderation.threadUnavailable')" />
      <BtgButton variant="secondary" @click="load()">{{ t('common.actions.tryAgain') }}</BtgButton>
    </BtgPanel>
    <template v-else>
      <div class="community-page__heading"><BtgPageTitle id="moderator-thread-title" focusable :title="state.thread.title" /><BtgBadge :variant="state.thread.state === 'HIDDEN' ? 'warning' : 'success'">{{ moderationStatus(state.thread.state) }}</BtgBadge></div>
      <div class="community-moderation-actions">
        <BtgButton :variant="state.thread.state === 'HIDDEN' ? 'secondary' : 'danger-secondary'" :leading-icon="state.thread.state === 'HIDDEN' ? 'show' : 'hide'" :loading="threadPending" @click="toggleThread">{{ state.thread.state === 'HIDDEN' ? t('community.moderation.unhideThread') : t('community.moderation.hideThread') }}</BtgButton>
      </div>
      <BtgPanel v-if="feedback" variant="subtle" padding="compact" :role="feedback.kind === 'error' ? 'alert' : 'status'"><p>{{ feedback.message }}</p></BtgPanel>
      <ol class="community-posts">
        <li v-for="post in state.thread.posts" :key="post.postId">
          <BtgPanel as="article" padding="compact" class="community-post">
            <div class="community-post__heading"><p class="community-post__meta"><span>{{ post.isOpeningPost ? t('community.moderation.openingPost') : t('community.moderation.reply') }}</span><time :datetime="post.createdAt">{{ date(post.createdAt, 'dateTime') }}</time></p><BtgBadge :variant="post.state === 'HIDDEN' ? 'warning' : 'success'">{{ moderationStatus(post.state) }}</BtgBadge></div>
            <p class="community-post__body">{{ post.body }}</p>
            <div v-if="!post.isOpeningPost" class="community-moderation-actions"><BtgButton :variant="post.state === 'HIDDEN' ? 'secondary' : 'danger-secondary'" :leading-icon="post.state === 'HIDDEN' ? 'show' : 'hide'" :loading="pendingPost === post.postId" :disabled="Boolean(pendingPost) && pendingPost !== post.postId" @click="togglePost(post.postId, post.state)">{{ post.state === 'HIDDEN' ? t('community.moderation.unhidePost') : t('community.moderation.hidePost') }}</BtgButton></div>
          </BtgPanel>
        </li>
      </ol>
      <CommunityPagination :limit="communityPageSize" :offset="postOffset" :total="state.thread.postTotal" :pending="threadPending || Boolean(pendingPost)" :copy="paginationCopy" @previous="load(Math.max(0, postOffset - communityPageSize))" @next="load(postOffset + communityPageSize)" />
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
import { communityModerationPath, communityPageSize, getCommunityThreadForModeration, hideCommunityPost, hideCommunityThread, unhideCommunityPost, unhideCommunityThread, type CommunityModeratorThread } from '../courses/community'
import { useApplicationFormatters } from '../i18n/formatters'

type State = { kind: 'loading' } | { kind: 'unavailable' } | { kind: 'ready'; thread: CommunityModeratorThread }
type Feedback = { kind: 'success' | 'error'; message: string }
const route = useRoute()
const auth = useAuth()
const { t } = useI18n()
const { date } = useApplicationFormatters()
const state = ref<State>({ kind: 'loading' })
const postOffset = ref(0)
const feedback = ref<Feedback>()
const threadPending = ref(false)
const pendingPost = ref('')
let active = true
let generation = 0
const paginationCopy = computed(() => ({ label: t('community.pagination.posts'), range: (from: number, to: number, total: number) => t('community.pagination.range', { from, to, total }), previous: t('community.pagination.previous'), next: t('community.pagination.next') }))
const course = () => typeof route.params.courseId === 'string' ? route.params.courseId : ''
const id = () => typeof route.params.threadId === 'string' ? route.params.threadId : ''

watch([() => route.params.courseId, () => route.params.threadId], () => { void load() }, { immediate: true })
onBeforeUnmount(() => { active = false; generation += 1 })
async function load(offset = 0) {
  const courseId = course()
  const threadId = id()
  const request = ++generation
  state.value = { kind: 'loading' }
  try {
    const thread = await getCommunityThreadForModeration(courseId, threadId, auth, offset)
    if (active && request === generation && courseId === course() && threadId === id()) { postOffset.value = offset; state.value = { kind: 'ready', thread } }
  } catch {
    if (active && request === generation && courseId === course() && threadId === id()) state.value = { kind: 'unavailable' }
  }
}
async function toggleThread() {
  if (state.value.kind !== 'ready' || threadPending.value) return
  threadPending.value = true
  feedback.value = undefined
  try {
    if (state.value.thread.state === 'HIDDEN') await unhideCommunityThread(course(), id(), auth)
    else await hideCommunityThread(course(), id(), auth)
    await load(postOffset.value)
    feedback.value = { kind: 'success', message: t('community.moderation.threadUpdated') }
  } catch { feedback.value = { kind: 'error', message: t('community.moderation.updateUnavailable') } } finally { threadPending.value = false }
}
async function togglePost(postId: string, postState: 'VISIBLE' | 'HIDDEN') {
  if (pendingPost.value) return
  pendingPost.value = postId
  feedback.value = undefined
  try {
    if (postState === 'HIDDEN') await unhideCommunityPost(course(), id(), postId, auth)
    else await hideCommunityPost(course(), id(), postId, auth)
    await load(postOffset.value)
    feedback.value = { kind: 'success', message: t('community.moderation.postUpdated') }
  } catch { feedback.value = { kind: 'error', message: t('community.moderation.updateUnavailable') } } finally { pendingPost.value = '' }
}
function moderationStatus(value: 'VISIBLE' | 'HIDDEN') { return value === 'HIDDEN' ? t('community.moderation.statusHidden') : t('community.moderation.statusVisible') }
</script>
