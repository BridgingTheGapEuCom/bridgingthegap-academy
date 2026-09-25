<template>
  <iframe
    v-if="safeLaunch"
    ref="frame"
    class="widget-runtime-frame"
    :src="launch.runtimeUrl"
    :title="title"
    sandbox="allow-scripts allow-same-origin"
    referrerpolicy="no-referrer"
  />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  isSafeRuntimeLaunch,
  runtimeMessageFrom,
  widgetRuntimeProtocol,
  widgetRuntimeProtocolVersion,
  type WidgetRuntimeLaunch,
} from '../plugins/runtime'

const props = defineProps<{ launch: WidgetRuntimeLaunch; title: string }>()
const emit = defineEmits<{ initialized: []; error: [] }>()
const frame = ref<HTMLIFrameElement>()
const safeLaunch = computed(() => isSafeRuntimeLaunch(props.launch))

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
          capabilities: props.launch.capabilities,
          expiresAt: props.launch.expiresAt,
        },
      },
      props.launch.runtimeOrigin,
    )
  } else if (message.type === 'RUNTIME_INITIALIZED') emit('initialized')
  else if (message.type === 'RUNTIME_ERROR') emit('error')
}

onMounted(() => window.addEventListener('message', receive))
onBeforeUnmount(() => window.removeEventListener('message', receive))
</script>

<style scoped>
.widget-runtime-frame { display: block; width: 100%; border: 0; }
</style>
