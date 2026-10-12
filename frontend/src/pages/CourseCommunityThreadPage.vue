<template>
  <BtgPageContainer as="section" class="community-page" aria-labelledby="thread-title">
    <RouterLink :to="communityPath(courseId())" class="community-page__back">
      <BtgIcon name="arrowLeft" decorative size="small" />
      {{ t('community.navigation.backToDiscussions') }}
    </RouterLink>

    <BtgPanel v-if="state.kind === 'loading'" role="status">
      <BtgPageTitle id="thread-title" :title="t('community.thread.loading')" />
    </BtgPanel>
    <BtgPanel v-else-if="state.kind === 'missing'" variant="subtle">
      <BtgPageTitle id="thread-title" :title="t('community.thread.unavailableTitle')" :description="t('community.thread.missing')" />
    </BtgPanel>
    <BtgPanel v-else-if="state.kind === 'unavailable'" class="community-page__state" role="alert">
      <BtgPageTitle id="thread-title" :title="t('community.thread.unavailableTitle')" :description="t('community.thread.unavailable')" />
      <BtgButton variant="secondary" @click="load()">{{ t('common.actions.tryAgain') }}</BtgButton>
    </BtgPanel>

    <template v-else>
      <BtgPageTitle id="thread-title" focusable :title="state.thread.title" />
      <ol class="community-posts">
        <li v-for="post in state.thread.posts" :key="post.postId">
          <BtgPanel as="article" padding="compact" class="community-post">
            <p class="community-post__meta">
              <span>{{ t('community.threads.participant') }}</span>
              <time :datetime="post.createdAt">{{ date(post.createdAt, 'dateTime') }}</time>
            </p>
            <p class="community-post__body">{{ post.body }}</p>
          </BtgPanel>
        </li>
      </ol>
      <CommunityPagination
        :limit="communityPageSize"
        :offset="postOffset"
        :total="state.thread.postTotal"
        :copy="paginationCopy"
        @previous="load(Math.max(0, postOffset - communityPageSize))"
        @next="load(postOffset + communityPageSize)"
      />

      <BtgSection>
        <template #heading>{{ t('community.thread.replyTitle') }}</template>
        <BtgPanel>
          <form class="community-form" @submit.prevent="reply">
            <BtgFormField :label="t('community.thread.messageLabel')" required v-slot="{ controlId, describedBy }">
              <BtgTextarea :id="controlId" v-model="body" :aria-describedby="describedBy" required maxlength="20000" rows="5" :disabled="replying" />
            </BtgFormField>
            <BtgPanel v-if="error" variant="subtle" padding="compact" role="alert">
              <p ref="errorElement" tabindex="-1">{{ error }}</p>
            </BtgPanel>
            <BtgPanel v-if="status" variant="subtle" padding="compact" role="status"><p>{{ status }}</p></BtgPanel>
            <div class="community-form__actions">
              <BtgButton type="submit" :loading="replying">{{ replying ? t('community.thread.replying') : t('community.thread.reply') }}</BtgButton>
            </div>
          </form>
        </BtgPanel>
      </BtgSection>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAuth } from '../auth/auth'
import { APIProblemError } from '../api/client'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgIcon from '../components/BtgIcon.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import BtgPageTitle from '../components/BtgPageTitle.vue'
import BtgPanel from '../components/BtgPanel.vue'
import BtgSection from '../components/BtgSection.vue'
import BtgTextarea from '../components/BtgTextarea.vue'
import CommunityPagination from '../components/CommunityPagination.vue'
import { communityPageSize, communityPath, createCommunityPost, getCommunityThread, type CommunityThread } from '../courses/community'
import { useApplicationFormatters } from '../i18n/formatters'

type State = { kind: 'loading' } | { kind: 'missing' } | { kind: 'unavailable' } | { kind: 'ready'; thread: CommunityThread }

const route = useRoute()
const auth = useAuth()
const { t } = useI18n()
const { date } = useApplicationFormatters()
const state = ref<State>({ kind: 'loading' })
const postOffset = ref(0)
const body = ref('')
const error = ref('')
const status = ref('')
const replying = ref(false)
const errorElement = ref<HTMLElement>()
let active = true
let generation = 0

const paginationCopy = computed(() => ({
  label: t('community.pagination.posts'),
  range: (from: number, to: number, total: number) => t('community.pagination.range', { from, to, total }),
  previous: t('community.pagination.previous'),
  next: t('community.pagination.next'),
}))
const courseId = () => typeof route.params.courseId === 'string' ? route.params.courseId : ''
const threadId = () => typeof route.params.threadId === 'string' ? route.params.threadId : ''

watch([() => route.params.courseId, () => route.params.threadId], () => { void load() }, { immediate: true })
onBeforeUnmount(() => { active = false; generation += 1 })

async function load(offset = 0) {
  const course = courseId()
  const thread = threadId()
  const request = ++generation
  state.value = { kind: 'loading' }
  try {
    const value = await getCommunityThread(course, thread, offset, auth)
    if (!active || request !== generation || course !== courseId() || thread !== threadId()) return
    postOffset.value = offset
    state.value = { kind: 'ready', thread: value }
  } catch (cause) {
    if (active && request === generation) state.value = cause instanceof APIProblemError && cause.status === 404 ? { kind: 'missing' } : { kind: 'unavailable' }
  }
}

async function reply() {
  if (replying.value) return
  replying.value = true
  error.value = ''
  status.value = ''
  try {
    const post = await createCommunityPost(courseId(), threadId(), body.value, auth)
    if (state.value.kind === 'ready') {
      const oldTotal = state.value.thread.postTotal
      const onLastPage = postOffset.value + state.value.thread.posts.length >= oldTotal
      state.value = { kind: 'ready', thread: {
        ...state.value.thread,
        posts: onLastPage ? [...state.value.thread.posts, post] : state.value.thread.posts,
        postTotal: oldTotal + 1,
      } }
      body.value = ''
      status.value = t('community.thread.posted')
    }
  } catch (cause) {
    error.value = cause instanceof APIProblemError && cause.problem?.code === 'community_disabled'
      ? t('community.create.disabled')
      : t('community.thread.postUnavailable')
    await nextTick()
    errorElement.value?.focus()
  } finally {
    replying.value = false
  }
}
</script>
