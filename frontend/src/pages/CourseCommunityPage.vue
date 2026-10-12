<template>
  <BtgPageContainer as="section" class="community-page" aria-labelledby="community-title">
    <RouterLink :to="coursePath(courseId())" class="community-page__back">
      <BtgIcon name="arrowLeft" decorative size="small" />
      {{ t('community.navigation.backToCourse') }}
    </RouterLink>

    <BtgPanel v-if="state.kind === 'loading'" role="status">
      <BtgPageTitle id="community-title" :title="t('community.page.loading')" />
    </BtgPanel>

    <BtgPanel v-else-if="state.kind === 'unavailable'" class="community-page__state" role="alert">
      <BtgPageTitle id="community-title" :title="t('community.page.unavailableTitle')" :description="t('community.page.unavailable')" />
      <BtgButton variant="secondary" @click="load()">{{ t('common.actions.tryAgain') }}</BtgButton>
    </BtgPanel>

    <template v-else>
      <div class="community-page__heading">
        <BtgPageTitle id="community-title" focusable :title="t('community.page.title')" :description="t('community.page.helper')" />
        <RouterLink v-if="canModerate" :to="communityModerationPath(courseId())" class="community-page__management-link">
          <BtgIcon name="settings" decorative size="small" />
          {{ t('community.page.moderationView') }}
        </RouterLink>
      </div>

      <BtgPanel v-if="state.mode === 'DISABLED'" as="section" variant="subtle" aria-labelledby="community-disabled">
        <h2 id="community-disabled">{{ t('community.page.disabledTitle') }}</h2>
        <p>{{ t('community.page.disabled') }}</p>
      </BtgPanel>

      <template v-else>
        <BtgSection>
          <template #heading>{{ t('community.create.title') }}</template>
          <BtgPanel>
            <form class="community-form" @submit.prevent="createThread">
              <BtgFormField :label="t('community.create.titleLabel')" required v-slot="{ controlId, describedBy }">
                <BtgTextInput :id="controlId" v-model="title" :aria-describedby="describedBy" required maxlength="240" :disabled="creating" />
              </BtgFormField>
              <BtgFormField :label="t('community.create.messageLabel')" required v-slot="{ controlId, describedBy }">
                <BtgTextarea :id="controlId" v-model="body" :aria-describedby="describedBy" required maxlength="20000" rows="5" :disabled="creating" />
              </BtgFormField>
              <BtgPanel v-if="formError" variant="subtle" padding="compact" role="alert">
                <p ref="formErrorElement" tabindex="-1">{{ formError }}</p>
              </BtgPanel>
              <div class="community-form__actions">
                <BtgButton type="submit" :loading="creating">{{ creating ? t('community.create.pending') : t('community.create.action') }}</BtgButton>
              </div>
            </form>
          </BtgPanel>
        </BtgSection>

        <BtgSection>
          <template #heading>{{ t('community.threads.title') }}</template>
          <BtgPanel v-if="state.page.threads.length === 0" variant="subtle">
            <p>{{ t('community.threads.empty') }}</p>
          </BtgPanel>
          <ol v-else class="community-threads">
            <li v-for="thread in state.page.threads" :key="thread.threadId">
              <BtgPanel as="article" padding="compact" class="community-thread-card">
                <RouterLink :to="communityThreadPath(courseId(), thread.threadId)" class="community-thread-card__title">{{ thread.title }}</RouterLink>
                <p class="community-thread-card__meta">
                  <span>{{ t('community.threads.participant') }}</span>
                  <span>{{ postCount(thread.postCount) }}</span>
                  <time :datetime="thread.updatedAt">{{ t('community.threads.updated', { date: date(thread.updatedAt, 'dateTime') }) }}</time>
                </p>
              </BtgPanel>
            </li>
          </ol>
          <CommunityPagination
            :limit="state.page.limit"
            :offset="state.page.offset"
            :total="state.page.total"
            :copy="paginationCopy"
            @previous="changePage(-1)"
            @next="changePage(1)"
          />
        </BtgSection>
      </template>
    </template>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
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
import BtgTextInput from '../components/BtgTextInput.vue'
import CommunityPagination from '../components/CommunityPagination.vue'
import { communityThreadPath, communityModerationPath, createCommunityThread, getCourseCommunity, getCommunityModerationProbe, listCommunityThreads, type CommunityThreadPage } from '../courses/community'
import { publishedCoursePath as coursePath } from '../courses/courses'
import { useApplicationFormatters } from '../i18n/formatters'

type State = { kind: 'loading' } | { kind: 'unavailable' } | { kind: 'ready'; mode: 'ENABLED' | 'DISABLED'; page: CommunityThreadPage }

const route = useRoute()
const router = useRouter()
const auth = useAuth()
const { t } = useI18n()
const { date } = useApplicationFormatters()
const state = ref<State>({ kind: 'loading' })
const title = ref('')
const body = ref('')
const formError = ref('')
const creating = ref(false)
const formErrorElement = ref<HTMLElement>()
const canModerate = ref(false)
let active = true
let generation = 0

const paginationCopy = computed(() => ({
  label: t('community.pagination.discussions'),
  range: (from: number, to: number, total: number) => t('community.pagination.range', { from, to, total }),
  previous: t('community.pagination.previous'),
  next: t('community.pagination.next'),
}))
const courseId = () => typeof route.params.courseId === 'string' ? route.params.courseId : ''

watch(() => route.params.courseId, () => { void load() }, { immediate: true })
onBeforeUnmount(() => { active = false; generation += 1 })

async function load(offset = 0) {
  const id = courseId()
  const request = ++generation
  state.value = { kind: 'loading' }
  canModerate.value = false
  try {
    const meta = await getCourseCommunity(id, auth)
    const page = meta.mode === 'ENABLED' ? await listCommunityThreads(id, offset, auth) : { threads: [], total: 0, limit: 20, offset: 0 }
    if (!active || request !== generation || id !== courseId()) return
    state.value = { kind: 'ready', mode: meta.mode, page }
    void getCommunityModerationProbe(id, auth).then((result) => {
      if (active && request === generation) canModerate.value = result.canModerate
    }).catch(() => {})
  } catch {
    if (active && request === generation) state.value = { kind: 'unavailable' }
  }
}

function changePage(delta: number) {
  if (state.value.kind === 'ready') void load(Math.max(0, state.value.page.offset + delta * state.value.page.limit))
}

async function createThread() {
  if (creating.value) return
  creating.value = true
  formError.value = ''
  try {
    const thread = await createCommunityThread(courseId(), title.value, body.value, auth)
    if (active) await router.push(communityThreadPath(courseId(), thread.threadId))
  } catch (error) {
    if (error instanceof APIProblemError && error.problem?.code === 'community_disabled') {
      formError.value = t('community.create.disabled')
      void load()
    } else {
      formError.value = t('community.create.unavailable')
    }
    await nextTick()
    formErrorElement.value?.focus()
  } finally {
    creating.value = false
  }
}

function postCount(count: number) {
  return t('community.counts.post', count)
}
</script>
