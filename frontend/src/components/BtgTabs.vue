<template>
  <div class="btg-tabs" role="tablist" :aria-label="label">
    <button
      v-for="tab in tabs"
      :id="`${tabId}-${tab.id}`"
      :key="tab.id"
      ref="tabButtons"
      class="btg-tabs__tab"
      type="button"
      role="tab"
      :aria-selected="tab.id === modelValue"
      :aria-controls="tab.panelId"
      :tabindex="tab.id === modelValue ? 0 : -1"
      :disabled="tab.disabled"
      @click="select(tab.id)"
      @keydown="onKeydown($event, tab.id)"
    >{{ tab.label }}</button>
  </div>
</template>

<script setup lang="ts">
import { nextTick, ref, useId } from 'vue'

type Tab = { id: string; label: string; panelId?: string; disabled?: boolean }

const props = defineProps<{ tabs: Tab[]; label: string; id?: string }>()
const modelValue = defineModel<string>({ required: true })
const emit = defineEmits<{ change: [id: string] }>()
const tabButtons = ref<HTMLButtonElement[]>([])
const tabId = props.id ?? `btg-tabs-${useId()}`

function enabledTabs() { return props.tabs.filter((tab) => !tab.disabled) }
function select(id: string) {
  if (!props.tabs.some((tab) => tab.id === id && !tab.disabled)) return
  modelValue.value = id
  emit('change', id)
}
async function move(currentId: string, direction: number) {
  const tabs = enabledTabs()
  const currentIndex = tabs.findIndex((tab) => tab.id === currentId)
  const target = tabs[(currentIndex + direction + tabs.length) % tabs.length]
  if (!target) return
  select(target.id)
  await nextTick()
  tabButtons.value.find((button) => button.id === `${tabId}-${target.id}`)?.focus()
}
async function moveTo(currentId: string, targetId: string) {
  if (currentId === targetId) return
  select(targetId)
  await nextTick()
  tabButtons.value.find((button) => button.id === `${tabId}-${targetId}`)?.focus()
}
function onKeydown(event: KeyboardEvent, currentId: string) {
  if (event.key === 'ArrowRight') { event.preventDefault(); void move(currentId, 1) }
  if (event.key === 'ArrowLeft') { event.preventDefault(); void move(currentId, -1) }
  if (event.key === 'Home') { event.preventDefault(); void moveTo(currentId, enabledTabs()[0]?.id ?? currentId) }
  if (event.key === 'End') { event.preventDefault(); void moveTo(currentId, enabledTabs().at(-1)?.id ?? currentId) }
}
</script>
