<template>
  <BtgPageContainer as="section" class="community-page" aria-labelledby="moderation-title">
    <RouterLink :to="communityPath(course())">Back to discussions</RouterLink>
    <h1 id="moderation-title">Moderation view</h1>
    <p v-if="error" role="alert">{{ error }}</p>
    <template v-else-if="page">
      <p v-if="page.threads.length === 0" class="community-page__empty">No discussions to moderate.</p>
      <ol v-else class="community-threads">
        <li v-for="thread in page.threads" :key="thread.threadId">
          <RouterLink :to="communityModeratorThreadPath(course(), thread.threadId)">{{ thread.title }}</RouterLink>
          <span v-if="thread.state === 'HIDDEN'"> — Hidden from participants</span>
        </li>
      </ol>
      <nav v-if="page.total > page.limit" aria-label="Moderation discussion pages">
        <BtgButton variant="secondary" :disabled="page.offset === 0" @click="changePage(-1)">Previous</BtgButton>
        <BtgButton variant="secondary" :disabled="page.offset + page.limit >= page.total" @click="changePage(1)">Next</BtgButton>
      </nav>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import { communityPath, communityModeratorThreadPath, listCommunityThreadsForModeration, type CommunityModeratorThreadPage } from '../courses/community'

const route = useRoute()
const page = ref<CommunityModeratorThreadPage>()
const error = ref('')
let active = true
let generation = 0

const course = () => String(route.params.courseId ?? '')

watch(() => route.params.courseId, () => { void load() }, { immediate: true })
onBeforeUnmount(() => { active = false; generation += 1 })

async function load(offset = 0) {
  const courseId = course()
  const request = ++generation
  error.value = ''
  try {
    const value = await listCommunityThreadsForModeration(courseId, offset)
    if (active && request === generation && courseId === course()) page.value = value
  } catch {
    if (active && request === generation && courseId === course()) error.value = 'Moderation is unavailable.'
  }
}

function changePage(delta: number) {
  if (page.value) void load(Math.max(0, page.value.offset + delta * page.value.limit))
}
</script>
