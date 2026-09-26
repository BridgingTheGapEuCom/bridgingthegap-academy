<template>
  <section class="dashboard-widget" :aria-label="`Dashboard widget ${placement.widgetId}`">
    <WidgetRuntimeFrame v-if="launch" :key="launch.context.runtimeInstanceId" :launch="launch" @error="unavailable = true" />
    <p v-else-if="unavailable" role="status">This widget is unavailable.</p>
    <p v-else role="status">Loading widget…</p>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import type { DashboardWidgetPlacement } from '../dashboard/dashboard'
import { launchDashboardWidget } from '../dashboard/dashboard'
import { isSafeRuntimeLaunch, type WidgetRuntimeLaunch } from '../plugins/runtime'
import WidgetRuntimeFrame from './WidgetRuntimeFrame.vue'

const props = defineProps<{ placement: DashboardWidgetPlacement }>()
const launch = ref<WidgetRuntimeLaunch>()
const unavailable = ref(false)
let active = true
let generation = 0

async function prepare() {
  const current = ++generation
  launch.value = undefined
  unavailable.value = false
  if (!props.placement.enabled) return
  try {
    const response = await launchDashboardWidget(props.placement.placementId)
    if (!active || current !== generation || !isSafeRuntimeLaunch(response)) return
    launch.value = response
  } catch {
    if (!active || current !== generation) return
    unavailable.value = true
  }
}

watch(() => `${props.placement.placementId}:${props.placement.revision}:${props.placement.enabled}`, () => { void prepare() }, { immediate: true })
onBeforeUnmount(() => { active = false; generation += 1 })
</script>

<style scoped>
.dashboard-widget { min-width: 0; }
</style>
