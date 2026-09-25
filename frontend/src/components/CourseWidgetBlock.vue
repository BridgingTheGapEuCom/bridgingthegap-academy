<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { apiClient } from '../api/client'
import { isCourseVersion, isPublishedCourseID, isLessonKey } from '../courses/courses'
import type { RenderableBlock } from '../lesson/content'
import { isSafeRuntimeLaunch, type WidgetRuntimeLaunch } from '../plugins/runtime'
import WidgetRuntimeFrame from './WidgetRuntimeFrame.vue'

const props = defineProps<{
  block: Extract<RenderableBlock, { type: 'PLUGIN_WIDGET' }>
  courseId?: string
  version?: string
  lessonKey?: string
}>()

const launch = ref<WidgetRuntimeLaunch>()
const unavailable = ref(false)
let generation = 0
let active = true

async function prepare() {
  const current = ++generation
  launch.value = undefined
  unavailable.value = false
  if (!props.courseId || !props.version || !props.lessonKey || !isPublishedCourseID(props.courseId) || !isCourseVersion(props.version) || !isLessonKey(props.lessonKey)) {
    unavailable.value = true
    return
  }
  try {
    const response = await apiClient.request<unknown>(`/api/courses/by-id/${encodeURIComponent(props.courseId)}/versions/${encodeURIComponent(props.version)}/lessons/${encodeURIComponent(props.lessonKey)}/blocks/${encodeURIComponent(props.block.key)}/widget-runtime`, {
      method: 'POST', cache: 'no-store', headers: { 'Content-Type': 'application/json' }, body: '',
    })
    if (!active || current !== generation || !isSafeRuntimeLaunch(response as WidgetRuntimeLaunch)) throw new Error('invalid launch')
    launch.value = response as WidgetRuntimeLaunch
  } catch (error) {
    if (!active || current !== generation) return
    // Disabled/revoked/missing releases deliberately look the same to learners.
    unavailable.value = true
  }
}

watch(() => `${props.courseId ?? ''}:${props.version ?? ''}:${props.lessonKey ?? ''}:${props.block.key}`, () => { void prepare() }, { immediate: true })
onBeforeUnmount(() => { active = false; generation += 1 })
</script>

<template>
  <section class="lesson-block lesson-widget" :aria-label="`Interactive widget: ${block.payload.widgetId}`">
    <WidgetRuntimeFrame v-if="launch" :launch="launch" @error="unavailable = true" />
    <p v-else-if="unavailable" role="status">This widget is unavailable.</p>
    <p v-else role="status">Loading widget…</p>
  </section>
</template>
