<template>
  <BtgPageContainer as="section" class="community-page" aria-labelledby="moderator-thread-title">
    <RouterLink :to="communityModerationPath(course())">Back to moderation</RouterLink>
    <div v-if="!thread" role="status">Loading discussion…</div>
    <template v-else>
      <h1 id="moderator-thread-title">{{ thread.title }}</h1>
      <p v-if="thread.state === 'HIDDEN'">Hidden from participants</p>
      <p class="sr-only" aria-live="polite">{{ status }}</p>
      <BtgButton :disabled="threadPending" @click="toggleThread">{{ thread.state === 'HIDDEN' ? 'Unhide thread' : 'Hide thread' }}</BtgButton>
      <ol class="community-posts">
        <li v-for="post in thread.posts" :key="post.postId">
          <article>
            <p v-if="post.state === 'HIDDEN'">Hidden from participants</p>
            <p class="community-posts__body">{{ post.body }}</p>
            <BtgButton v-if="!post.isOpeningPost" :disabled="pendingPost === post.postId" @click="togglePost(post.postId, post.state)">{{ post.state === 'HIDDEN' ? 'Unhide post' : 'Hide post' }}</BtgButton>
          </article>
        </li>
      </ol>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import { communityModerationPath, getCommunityThreadForModeration, hideCommunityPost, hideCommunityThread, unhideCommunityPost, unhideCommunityThread, type CommunityModeratorThread } from '../courses/community'

const route = useRoute()
const thread = ref<CommunityModeratorThread>()
const status = ref('')
const threadPending = ref(false)
const pendingPost = ref('')
let active = true
let generation = 0

const course = () => String(route.params.courseId ?? '')
const id = () => String(route.params.threadId ?? '')

async function load() {
  const courseId = course()
  const threadId = id()
  const request = ++generation
  try {
    const value = await getCommunityThreadForModeration(courseId, threadId)
    if (active && request === generation && courseId === course() && threadId === id()) thread.value = value
  } catch {
    if (active && request === generation && courseId === course() && threadId === id()) status.value = 'Discussion unavailable.'
  }
}

watch([() => route.params.courseId, () => route.params.threadId], load, { immediate: true })
onBeforeUnmount(() => { active = false; generation += 1 })

async function toggleThread() {
  if (!thread.value || threadPending.value) return
  threadPending.value = true
  try {
    if (thread.value.state === 'HIDDEN') await unhideCommunityThread(course(), id())
    else await hideCommunityThread(course(), id())
    await load()
    status.value = 'Thread visibility updated.'
  } catch {
    status.value = 'Visibility could not be updated.'
  } finally {
    threadPending.value = false
  }
}

async function togglePost(postId: string, state: string) {
  if (pendingPost.value) return
  pendingPost.value = postId
  try {
    if (state === 'HIDDEN') await unhideCommunityPost(course(), id(), postId)
    else await hideCommunityPost(course(), id(), postId)
    await load()
    status.value = 'Post visibility updated.'
  } catch {
    status.value = 'Visibility could not be updated.'
  } finally {
    pendingPost.value = ''
  }
}
</script>
