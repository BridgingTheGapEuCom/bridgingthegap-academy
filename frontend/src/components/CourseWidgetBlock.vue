<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { apiClient } from '../api/client'
import { isCourseVersion, isPublishedCourseID, isLessonKey } from '../courses/courses'
import { isAuthoringDraftID, launchDraftPreviewWidgetRuntime } from '../authoring/authoring'
import type { RenderableBlock } from '../lesson/content'
import { isSafeRuntimeLaunch, type WidgetRuntimeLaunch } from '../plugins/runtime'
import WidgetRuntimeFrame from './WidgetRuntimeFrame.vue'

const props = defineProps<{
  block: Extract<RenderableBlock, { type: 'PLUGIN_WIDGET' }>
  courseId?: string
  version?: string
  lessonKey?: string
  draftId?: string
  draftLessonId?: string
}>()

const launch = ref<WidgetRuntimeLaunch>()
const unavailable = ref(false)
let generation = 0
let active = true

async function prepare() {
  const current = ++generation
  launch.value = undefined
  unavailable.value = false
  const draftPreview = props.draftId && props.draftLessonId && isAuthoringDraftID(props.draftId) && isAuthoringDraftID(props.draftLessonId)
  const published = props.courseId && props.version && props.lessonKey && isPublishedCourseID(props.courseId) && isCourseVersion(props.version) && isLessonKey(props.lessonKey)
  if (!draftPreview && !published) {
    unavailable.value = true
    return
  }
  try {
    const response = draftPreview
      ? await launchDraftPreviewWidgetRuntime(props.draftId!, props.draftLessonId!, props.block.key)
      : await apiClient.request<unknown>(
          `/api/courses/by-id/${encodeURIComponent(props.courseId!)}/versions/${encodeURIComponent(props.version!)}/lessons/${encodeURIComponent(props.lessonKey!)}/blocks/${encodeURIComponent(props.block.key)}/widget-runtime`,
          { method: 'POST', cache: 'no-store' },
        )
    const candidate = response as WidgetRuntimeLaunch
    const isExpectedPreview = !draftPreview || (
      candidate.draftPreviewContext?.contextType === 'DRAFT_PREVIEW'
      && candidate.draftPreviewContext.placementKey === props.block.key
      && candidate.courseContext === undefined
      && candidate.dashboardContext === undefined
      && candidate.capabilities.includes('widget.course.preview.context.read')
    )
    if (!active || current !== generation || !isSafeRuntimeLaunch(candidate) || !isExpectedPreview) throw new Error('invalid launch')
    launch.value = candidate
  } catch (error) {
    if (!active || current !== generation) return
    // Disabled/revoked/missing releases deliberately look the same to learners.
    unavailable.value = true
  }
}

watch(() => `${props.courseId ?? ''}:${props.version ?? ''}:${props.lessonKey ?? ''}:${props.draftId ?? ''}:${props.draftLessonId ?? ''}:${props.block.key}`, () => { void prepare() }, { immediate: true })
onBeforeUnmount(() => { active = false; generation += 1 })
</script>

<template>
  <section class="lesson-block lesson-widget" :aria-label="`Interactive widget: ${block.payload.widgetId}`">
    <WidgetRuntimeFrame v-if="launch" :launch="launch" @error="unavailable = true" />
    <p v-else-if="unavailable" role="status">{{ draftId ? 'Widget unavailable in preview.' : 'This widget is unavailable.' }}</p>
    <p v-else role="status">Loading widget…</p>
  </section>
</template>
