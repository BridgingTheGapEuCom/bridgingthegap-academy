<template>
  <fieldset class="objectives-editor" :aria-describedby="describedBy">
    <legend>{{ label }} <span aria-hidden="true">*</span><span class="sr-only"> required</span></legend>
    <p v-if="description" :id="descriptionID" class="objectives-editor__description">{{ description }}</p>
    <BtgOrderedList :aria-label="label">
      <BtgOrderedRow v-for="(_, index) in modelValue" :key="index" :index="index" :class="{ 'is-dragging': draggingIndex === index, 'is-drop-before': insertionIndex === index && draggingIndex !== index, 'is-drop-after': insertionIndex === index + 1 && draggingIndex !== index }" :data-objective-index="index">
        <template #handle><BtgDragHandle :id="`${fieldID}-reorder-${index}`" :label="reorderLabel(index + 1)" :aria-pressed="keyboardIndex === index" :disabled="disabled" @pointerdown="startPointerReorder($event, index)" @keydown="keyboardReorder($event, index)" /></template>
        <BtgTextInput :model-value="modelValue[index]" :disabled="disabled" :aria-label="`${objectiveName} ${index + 1}`" :aria-invalid="Boolean(error) || undefined" placeholder="Enter a learning objective" @update:model-value="update(index, $event)" />
        <template #actions><BtgButton type="button" variant="secondary" leading-icon="delete" class="objectives-editor__remove" :disabled="disabled || modelValue.length === 1" :aria-label="removeLabelFor(index + 1)" @click="remove(index)">{{ removeText }}</BtgButton></template>
      </BtgOrderedRow>
    </BtgOrderedList>
    <p v-if="error" :id="errorID" class="objectives-editor__error">{{ error }}</p>
    <BtgButton type="button" variant="secondary" class="objectives-editor__add" :disabled="disabled" @click="add">{{ addLabel }}</BtgButton>
    <p class="sr-only" aria-live="polite">{{ announcement }}</p>
  </fieldset>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, useId } from 'vue'
import BtgButton from './BtgButton.vue'
import BtgTextInput from './BtgTextInput.vue'
import BtgDragHandle from './BtgDragHandle.vue'
import BtgOrderedList from './BtgOrderedList.vue'
import BtgOrderedRow from './BtgOrderedRow.vue'

const props = withDefaults(defineProps<{ modelValue: string[]; label?: string; description?: string; error?: string; disabled?: boolean; itemLabel?: string; removeLabel?: string; addLabel?: string; removeText?: string; reorderLabel?: (position: number) => string; removeLabelFor?: (position: number) => string }>(), {
  label: 'Learning objectives', description: undefined, error: undefined, disabled: false, itemLabel: undefined, removeLabel: 'objective', addLabel: '+ Add objective', removeText: 'Remove', reorderLabel: undefined, removeLabelFor: undefined,
})
const emit = defineEmits<{ 'update:modelValue': [value: string[]] }>()
const announcement = ref('')
const draggingIndex = ref<number>()
const insertionIndex = ref<number>()
const keyboardIndex = ref<number>()
const fieldID = `objectives-${useId()}`
const descriptionID = `${fieldID}-description`
const errorID = `${fieldID}-error`
const describedBy = computed(() => [props.description ? descriptionID : '', props.error ? errorID : ''].filter(Boolean).join(' ') || undefined)
const objectiveName = computed(() => props.itemLabel ?? (props.label.endsWith('s') ? props.label.slice(0, -1) : props.label))
function reorderLabel(position: number) { return props.reorderLabel?.(position) ?? `Reorder ${objectiveName.value} ${position}` }
function removeLabelFor(position: number) { return props.removeLabelFor?.(position) ?? `Remove ${props.removeLabel} ${position}` }
let pointerID: number | undefined
let sourceIndex = -1

onBeforeUnmount(clearPointerReorder)

function update(index: number, value: string) { const next = [...props.modelValue]; next[index] = value; emit('update:modelValue', next) }
function add() { emit('update:modelValue', [...props.modelValue, '']); announcement.value = `${capitalize(objectiveName.value)} ${props.modelValue.length + 1} added.` }
function remove(index: number) { if (props.modelValue.length === 1) return; emit('update:modelValue', props.modelValue.filter((_, currentIndex) => currentIndex !== index)); announcement.value = `${capitalize(objectiveName.value)} removed.` }
function capitalize(value: string) { return `${value.charAt(0).toUpperCase()}${value.slice(1)}` }
function focusHandle(index: number) { document.getElementById(`${fieldID}-reorder-${index}`)?.focus() }
function move(from: number, to: number, focus = false) {
  if (props.disabled || from < 0 || to < 0 || from >= props.modelValue.length || to >= props.modelValue.length || from === to) return false
  const next = [...props.modelValue]; const [item] = next.splice(from, 1)
  if (item === undefined) return false
  next.splice(to, 0, item); emit('update:modelValue', next)
  announcement.value = `${capitalize(objectiveName.value)} moved to position ${to + 1} of ${next.length}.`
  if (focus) void nextTick(() => focusHandle(to))
  return true
}
function clearPointerReorder() {
  document.removeEventListener('pointermove', updatePointerReorder); document.removeEventListener('pointerup', finishPointerReorder); document.removeEventListener('pointercancel', cancelPointerReorder)
  pointerID = undefined; sourceIndex = -1; draggingIndex.value = undefined; insertionIndex.value = undefined
}
function startPointerReorder(event: PointerEvent, index: number) {
  if (props.disabled || event.button !== 0) return
  event.preventDefault(); keyboardIndex.value = undefined; pointerID = event.pointerId; sourceIndex = index; draggingIndex.value = index; insertionIndex.value = index
  document.addEventListener('pointermove', updatePointerReorder); document.addEventListener('pointerup', finishPointerReorder); document.addEventListener('pointercancel', cancelPointerReorder)
}
function updatePointerReorder(event: PointerEvent) {
  if (event.pointerId !== pointerID || draggingIndex.value === undefined) return
  const row = document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>('[data-objective-index]'); const target = Number(row?.dataset.objectiveIndex)
  if (!row || !Number.isInteger(target)) return
  const bounds = row.getBoundingClientRect(); insertionIndex.value = event.clientY < bounds.top + bounds.height / 2 ? target : target + 1
}
function finishPointerReorder(event: PointerEvent) { if (event.pointerId !== pointerID) return; const from = sourceIndex; const insertion = insertionIndex.value ?? from; clearPointerReorder(); move(from, insertion > from ? insertion - 1 : insertion, true) }
function cancelPointerReorder(event: PointerEvent) { if (event.pointerId !== pointerID) return; const from = sourceIndex; clearPointerReorder(); if (from >= 0) void nextTick(() => focusHandle(from)) }
function keyboardReorder(event: KeyboardEvent, index: number) {
  if (props.disabled) return
  if (event.key === ' ' || event.key === 'Enter') { event.preventDefault(); keyboardIndex.value = keyboardIndex.value === index ? undefined : index; announcement.value = keyboardIndex.value === index ? `Reordering ${objectiveName.value} ${index + 1}. Use the arrow keys to move it, or Escape to finish.` : `Finished reordering ${objectiveName.value}.`; return }
  if (event.key === 'Escape' && keyboardIndex.value === index) { event.preventDefault(); keyboardIndex.value = undefined; announcement.value = `Finished reordering ${objectiveName.value}.`; return }
  if (keyboardIndex.value !== index || (event.key !== 'ArrowUp' && event.key !== 'ArrowDown')) return
  event.preventDefault(); move(index, index + (event.key === 'ArrowUp' ? -1 : 1), true); keyboardIndex.value = undefined
}
</script>

<style scoped>
.objectives-editor { display: grid; min-width: 0; gap: var(--authoring-field-gap); margin: 0; border: 0; padding: 0; }
.objectives-editor legend { padding: 0; font-weight: var(--btg-font-weight-strong); }.objectives-editor__description { margin: 0; color: var(--btg-color-text-secondary); font-size: var(--btg-font-size-small); }
.objectives-editor ol { display: grid; gap: var(--btg-space-2); margin: 0; padding: 0; list-style: none; }
.objectives-editor li { position: relative; display: grid; grid-template-columns: 2.25rem 2.25rem minmax(0, 1fr) auto; align-items: center; gap: var(--btg-space-3); max-width: none; border-bottom: var(--btg-border-width) solid var(--authoring-border-subtle); padding-block: var(--btg-space-2); }.objectives-editor li:last-child { border-bottom: 0; }.objectives-editor li.is-dragging { opacity: .55; }
.objectives-editor li.is-drop-before::before, .objectives-editor li.is-drop-after::after { position: absolute; right: 0; left: 0; height: .15rem; background: var(--btg-color-text); content: ''; }.objectives-editor li.is-drop-before::before { top: calc(-1 * var(--btg-space-2)); }.objectives-editor li.is-drop-after::after { bottom: calc(-1 * var(--btg-space-2)); }
.objectives-editor__drag-handle { display: inline-grid; width: 2.25rem; min-height: 2.5rem; place-items: center; border: var(--btg-border-width) solid transparent; border-radius: var(--btg-radius-control); background: transparent; color: var(--btg-color-border-strong); cursor: grab; touch-action: none; }.objectives-editor__drag-handle:hover, .objectives-editor__drag-handle:focus-visible, .objectives-editor__drag-handle[aria-pressed='true'] { border-color: var(--btg-color-border-strong); background: var(--btg-color-surface-muted); color: var(--btg-color-text); }.objectives-editor__drag-handle:active { cursor: grabbing; }.objectives-editor__drag-handle svg { width: 1rem; height: 1rem; fill: none; stroke: currentcolor; stroke-linecap: round; stroke-width: 3; }
.objectives-editor__number { display: grid; width: 2.25rem; min-height: 2.5rem; place-items: center; color: var(--btg-color-text-secondary); font-size: var(--btg-font-size-small); font-weight: var(--btg-font-weight-strong); letter-spacing: .06em; }
.objectives-editor__remove { min-height: 2.75rem; padding-inline: var(--btg-space-3); }.objectives-editor__add { width: fit-content; margin-top: var(--btg-space-1); }.objectives-editor__error { margin: 0; color: var(--btg-color-danger); font-weight: var(--btg-font-weight-medium); }
@media (max-width: 34rem) { .objectives-editor li { grid-template-columns: 2rem 2rem minmax(0, 1fr); gap: var(--btg-space-2); }.objectives-editor__drag-handle, .objectives-editor__number { width: 2rem; min-height: 2.25rem; }.objectives-editor__remove { grid-column: 3; width: fit-content; } }
</style>
