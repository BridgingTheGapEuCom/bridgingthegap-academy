<template>
  <div ref="root" class="language-picker" @focusout="closeWhenFocusLeaves">
    <input
      :id="id"
      ref="input"
      class="language-picker__input"
      type="text"
      role="combobox"
      :value="inputValue"
      :disabled="disabled"
      :readonly="!open"
      :aria-controls="listboxID"
      :aria-describedby="describedBy"
      :aria-expanded="open"
      :aria-activedescendant="activeOptionID"
      :aria-invalid="invalid || undefined"
      aria-autocomplete="list"
      autocomplete="off"
      placeholder="Search by language or tag"
      @focus="openPicker"
      @input="search"
      @keydown="handleKeydown"
    />
    <button
      class="language-picker__toggle"
      type="button"
      :disabled="disabled"
      :aria-label="open ? 'Close language options' : 'Show language options'"
      :aria-expanded="open"
      :aria-controls="listboxID"
      tabindex="-1"
      @mousedown.prevent
      @click="togglePicker"
    >
      <span aria-hidden="true">▾</span>
    </button>
    <ul v-if="open" :id="listboxID" class="language-picker__options" role="listbox" aria-label="Language options">
      <li v-if="!options.length" class="language-picker__empty" role="presentation">No matching languages.</li>
      <li
        v-for="(option, index) in options"
        :id="optionID(index)"
        :key="option.tag"
        class="language-picker__option"
        :class="{ 'language-picker__option--active': index === activeIndex }"
        role="option"
        :aria-selected="option.tag === modelValue"
        @mousedown.prevent
        @click="select(option)"
      >
        {{ option.label }}
      </li>
    </ul>
    <p class="sr-only" aria-live="polite">{{ statusMessage }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { canonicalLanguageTag, filterLanguageOptions, languageLabel, type LanguageOption } from '../i18n/languages'

const props = defineProps<{
  id: string
  modelValue: string
  disabled?: boolean
  invalid?: boolean
  describedBy?: string
}>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const input = ref<HTMLInputElement>()
const root = ref<HTMLElement>()
const open = ref(false)
const query = ref('')
const activeIndex = ref(-1)
const statusMessage = ref('')
const listboxID = `${props.id}-options`

const options = computed<LanguageOption[]>(() => {
  const matches = filterLanguageOptions(query.value)
  const canonical = canonicalLanguageTag(query.value)
  if (canonical && !matches.some((option) => option.tag === canonical)) {
    return [...matches, { tag: canonical, label: `Use language tag “${canonical}”`, search: canonical.toLocaleLowerCase() }]
  }
  return matches
})
const inputValue = computed(() => open.value ? query.value : (props.modelValue ? languageLabel(props.modelValue) : ''))
const activeOptionID = computed(() => activeIndex.value >= 0 ? optionID(activeIndex.value) : undefined)

watch(options, (nextOptions) => {
  if (activeIndex.value >= nextOptions.length) activeIndex.value = nextOptions.length - 1
})

onMounted(() => document.addEventListener('pointerdown', closeWhenPointerLeaves))
onBeforeUnmount(() => document.removeEventListener('pointerdown', closeWhenPointerLeaves))

function optionID(index: number) { return `${props.id}-option-${index}` }
function openPicker() {
  if (props.disabled) return
  open.value = true
  query.value = ''
  activeIndex.value = -1
  statusMessage.value = `${options.value.length} language options available.`
}
function closePicker() {
  open.value = false
  activeIndex.value = -1
  query.value = ''
}
function closeWhenFocusLeaves(event: FocusEvent) {
  const nextTarget = event.relatedTarget
  if (nextTarget instanceof Node && root.value?.contains(nextTarget)) return
  closePicker()
}
function closeWhenPointerLeaves(event: PointerEvent) {
  if (event.target instanceof Node && !root.value?.contains(event.target)) closePicker()
}
function togglePicker() {
  if (open.value) closePicker()
  else {
    openPicker()
    void nextTick(() => input.value?.focus())
  }
}
function search(event: Event) {
  query.value = (event.target as HTMLInputElement).value
  activeIndex.value = options.value.length ? 0 : -1
  statusMessage.value = options.value.length ? `${options.value.length} language options available.` : 'No matching languages.'
}
function select(option: LanguageOption) {
  emit('update:modelValue', option.tag)
  statusMessage.value = `${languageLabel(option.tag)} selected.`
  closePicker()
}
function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && open.value) {
    event.preventDefault()
    closePicker()
    return
  }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    if (!open.value) openPicker()
    if (!options.value.length) return
    if (activeIndex.value === -1) {
      activeIndex.value = event.key === 'ArrowDown' ? 0 : options.value.length - 1
      return
    }
    const direction = event.key === 'ArrowDown' ? 1 : -1
    activeIndex.value = (activeIndex.value + direction + options.value.length) % options.value.length
    return
  }
  if (event.key === 'Enter' && open.value && activeIndex.value >= 0) {
    event.preventDefault()
    const option = options.value[activeIndex.value]
    if (option) select(option)
  }
}
</script>

<style scoped>
.language-picker { position: relative; min-width: 0; }
.language-picker__input { width: 100%; padding-inline-end: 2.75rem; }
.language-picker__toggle { position: absolute; top: 0; right: 0; display: grid; width: 2.75rem; height: 100%; place-items: center; border: 0; border-inline-start: 1px solid var(--btg-color-border); border-radius: 0 var(--btg-radius-control) var(--btg-radius-control) 0; background: transparent; color: var(--btg-color-text); }
.language-picker__toggle:hover:not(:disabled) { background: var(--btg-color-surface-muted); }
.language-picker__options { position: absolute; z-index: 2; display: grid; max-height: 16rem; width: 100%; margin: var(--btg-space-1) 0 0; overflow-y: auto; border: 1px solid var(--btg-color-border-strong); border-radius: var(--btg-radius-control); background: var(--btg-color-surface-elevated); box-shadow: var(--btg-shadow-elevated); padding: var(--btg-space-1); list-style: none; }
.language-picker__option, .language-picker__empty { padding: var(--btg-space-2) var(--btg-space-3); }
.language-picker__option { cursor: pointer; color: var(--btg-color-text); }
.language-picker__option:hover, .language-picker__option--active { background: var(--btg-color-surface-muted); }
.language-picker__empty { color: var(--btg-color-text-secondary); }
</style>
