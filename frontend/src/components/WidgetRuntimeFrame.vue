<template>
  <div class="widget-runtime">
    <p v-if="status === 'loading'" role="status">Loading widget…</p>
    <p v-else-if="status === 'failed' || status === 'unavailable'" role="alert">
      This widget is unavailable.
    </p>
    <iframe
      v-if="safeLaunch && status !== 'failed' && status !== 'unavailable'"
      ref="frame"
      class="widget-runtime-frame"
      :src="launch.runtimeUrl"
      :title="launch.widgetName"
      sandbox="allow-scripts allow-same-origin"
      referrerpolicy="no-referrer"
      @error="fail('unavailable')"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  isSafeRuntimeLaunch,
  runtimeMessageFrom,
  widgetRuntimeProtocol,
  widgetRuntimeProtocolVersion,
  type WidgetRuntimeLaunch,
} from '../plugins/runtime'

const props = defineProps<{ launch: WidgetRuntimeLaunch }>()
const emit = defineEmits<{ initialized: []; error: [] }>()
const frame = ref<HTMLIFrameElement>()
const safeLaunch = computed(() => isSafeRuntimeLaunch(props.launch))
const status = ref<'loading' | 'ready' | 'failed' | 'unavailable'>(
  safeLaunch.value ? 'loading' : 'unavailable',
)
let initializationTimer: ReturnType<typeof setTimeout> | undefined

function clearInitializationTimer() {
  if (initializationTimer !== undefined) clearTimeout(initializationTimer)
  initializationTimer = undefined
}

function startInitializationTimer() {
  clearInitializationTimer()
  if (!safeLaunch.value) {
    status.value = 'unavailable'
    return
  }
  status.value = 'loading'
  initializationTimer = setTimeout(() => fail('failed'), 10_000)
}

function fail(next: 'failed' | 'unavailable') {
  clearInitializationTimer()
  status.value = next
  emit('error')
}

function receive(event: MessageEvent) {
  if (!frame.value) return
  const message = runtimeMessageFrom(event, frame.value, props.launch)
  if (!message) return
  if (message.type === 'WIDGET_READY') {
    frame.value.contentWindow?.postMessage(
      {
        protocol: widgetRuntimeProtocol,
        version: widgetRuntimeProtocolVersion,
        type: 'RUNTIME_INIT',
        runtimeInstanceId: props.launch.context.runtimeInstanceId,
        payload: {
          token: props.launch.token,
          context: props.launch.context,
          courseContext: props.launch.courseContext,
          capabilities: props.launch.capabilities,
          expiresAt: props.launch.expiresAt,
        },
      },
      props.launch.runtimeOrigin,
    )
  } else if (message.type === 'RUNTIME_INITIALIZED') {
    clearInitializationTimer()
    status.value = 'ready'
    emit('initialized')
  } else if (message.type === 'RUNTIME_ERROR') fail('failed')
}

watch(() => props.launch.context.runtimeInstanceId, startInitializationTimer)
onMounted(() => {
  window.addEventListener('message', receive)
  startInitializationTimer()
})
onBeforeUnmount(() => {
  clearInitializationTimer()
  window.removeEventListener('message', receive)
})
</script>

<style scoped>
.widget-runtime { min-width: 0; }
.widget-runtime-frame { display: block; width: 100%; border: 0; }
</style>
