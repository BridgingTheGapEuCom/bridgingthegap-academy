<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { APIProblemError } from '../api/client'
import { preserveFocusAfterRemoval } from '../authoring/focus'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { getAuthoringLesson, listAuthoringCourseWidgets, replaceAuthoringLessonContent, type AuthoringCourseWidget, type AuthoringLessonContent, type AuthoringLessonContentMutation, type AuthoringLessonDetail } from '../authoring/authoring'
import { contentEditorError, contentEditorLimits, contentFingerprint, copyContent, createContentBlock, editableBlockTypes, type CanonicalBlock, type EditableBlockType } from '../authoring/contentEditor'
import { richTextPlainText } from '../authoring/richText'
import { decodeLessonContent } from '../lesson/content'
import { draftAssetURL } from '../lesson/assets'
import AuthoringContentBlock from './AuthoringContentBlock.vue'
import AuthoringDirtyActionBar from './AuthoringDirtyActionBar.vue'
import BtgButton from './BtgButton.vue'
import BtgDragHandle from './BtgDragHandle.vue'
import BtgIcon from './BtgIcon.vue'
import BtgIconButton from './BtgIconButton.vue'
import BtgToolbar from './BtgToolbar.vue'
import BtgDialog from './BtgDialog.vue'
import type { BtgIconName } from './btg-icon-names'

const props = defineProps<{ draftId: string; lesson: AuthoringLessonDetail }>()
const emit = defineEmits<{ saved: [result: AuthoringLessonContentMutation]; replaceLesson: [lesson: AuthoringLessonDetail]; preview: []; unavailable: [] }>()
const document = ref<AuthoringLessonContent>(copyContent(props.lesson.content))
const baseline = ref(contentFingerprint(props.lesson.content))
const savedContent = ref<AuthoringLessonContent>(copyContent(props.lesson.content))
const supported = ref(false)
const pickerOpen = ref(false)
const previewWarning = ref<HTMLDialogElement>()
const editingKey = ref<string>()
const editingBlock = ref<CanonicalBlock>()
const editingOriginal = ref<CanonicalBlock>()
const actionsIndex = ref<number>()
const pickerTrigger = ref<HTMLElement>()
const draggingKey = ref<string>()
const insertionIndex = ref<number>()
const keyboardReorderKey = ref<string>()
const reorderAnnouncement = ref('')
const saving = ref(false)
const reloading = ref(false)
const conflict = ref(false)
const formError = ref<string>()
const message = ref<string>()
const courseWidgets = ref<AuthoringCourseWidget[]>([])
const dirty = computed(() => contentFingerprint(document.value) !== baseline.value)
const { t } = useI18n()
const pickerGroups = [
  { id: 'content', types: ['TEXT', 'HEADING', 'IMAGE', 'VIDEO', 'AUDIO', 'DOWNLOAD', 'CODE', 'CALLOUT', 'QUOTE', 'DIVIDER'] as EditableBlockType[] },
  { id: 'interactive', types: ['KNOWLEDGE_CHECK', 'PLUGIN_WIDGET'] as EditableBlockType[] },
] as const
const pickerDetails: Record<EditableBlockType, { description: string; icon: string }> = {
  TEXT: { description: 'Add paragraphs of written content.', icon: 'M4 5h16M4 10h16M4 15h12M4 20h16' },
  HEADING: { description: 'Add a section heading.', icon: 'M5 4v16M19 4v16M5 12h14' },
  IMAGE: { description: 'Add an image from Academy assets.', icon: 'M4 5h16v14H4zM6 16l4-4 3 3 2-2 3 3' },
  VIDEO: { description: 'Add video with accessible media details.', icon: 'M4 6h11v12H4zM15 10l5-3v10l-5-3z' },
  AUDIO: { description: 'Add audio and its transcript.', icon: 'M4 10h4l5-4v12l-5-4H4zM16 9c1 .8 1 5.2 0 6M19 6c3 3 3 9 0 12' },
  DOWNLOAD: { description: 'Add a downloadable Academy asset.', icon: 'M12 3v11M8 10l4 4 4-4M5 20h14' },
  CODE: { description: 'Add a formatted code example.', icon: 'M9 6 4 12l5 6M15 6l5 6-5 6' },
  CALLOUT: { description: 'Highlight an important idea.', icon: 'M12 4 3 20h18L12 4zM12 9v4M12 17h.01' },
  QUOTE: { description: 'Add a quoted source or insight.', icon: 'M7 7H4v6h5V9H7zm9 0h-3v6h5V9h-2z' },
  DIVIDER: { description: 'Separate two parts of the lesson.', icon: 'M4 12h16' },
  KNOWLEDGE_CHECK: { description: 'Add an inline learner assessment.', icon: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM9.5 9a2.5 2.5 0 1 1 4.5 1.5c-1 .8-2 1.2-2 2.5M12 17h.01' },
  PLUGIN_WIDGET: { description: 'Add an available Course widget.', icon: 'M8 3v4M16 3v4M3 8h4M17 8h4M8 17v4M16 17v4M3 16h4M17 16h4M8 8h8v8H8z' },
}
const pickerOptions = computed(() => new Map(editableBlockTypes.map((entry) => [entry.type, { ...entry, ...pickerDetails[entry.type] }])))
let loadedLessonID = ''
let requestVersion = 0
let active = true
let dragPointerID: number | undefined
let dragSourceIndex = -1

async function loadCourseWidgets() {
  const generation = requestVersion
  try {
    const result = await listAuthoringCourseWidgets(props.draftId)
    if (active && generation === requestVersion) courseWidgets.value = result.widgets
  } catch {
    if (active && generation === requestVersion) courseWidgets.value = []
  }
}


function initialize(content: AuthoringLessonContent) {
  document.value = copyContent(content)
  savedContent.value = copyContent(content)
  baseline.value = contentFingerprint(content)
  const decoded = decodeLessonContent(content)
  supported.value = decoded.kind === 'ready' && decoded.blocks.every((block) => block.type !== 'UNSUPPORTED')
  conflict.value = false
  formError.value = undefined
  message.value = undefined
  editingKey.value = undefined
  editingBlock.value = undefined
  editingOriginal.value = undefined
  draggingKey.value = undefined
  insertionIndex.value = undefined
  keyboardReorderKey.value = undefined
  reorderAnnouncement.value = ''
}
const captureScope = useAuthoringAsyncScope(() => `${props.draftId}/${props.lesson.id}`)

watch(() => [props.lesson.id, contentFingerprint(props.lesson.content)], () => {
  if (loadedLessonID !== props.lesson.id) {
    requestVersion += 1
    loadedLessonID = props.lesson.id
    saving.value = false
    reloading.value = false
    initialize(props.lesson.content)
		void loadCourseWidgets()
  } else if (contentFingerprint(props.lesson.content) !== baseline.value) {
    if (dirty.value) conflict.value = true
    else initialize(props.lesson.content)
  }
}, { immediate: true })
onMounted(() => globalThis.document.addEventListener('pointerdown', closeActionsOnOutsidePointer, true))
onBeforeUnmount(() => {
  active = false
  clearPointerReorder()
  globalThis.document.removeEventListener('pointerdown', closeActionsOnOutsidePointer, true)
})

function label(type: string) { return (editableBlockTypes.find((entry) => entry.type === type)?.label ?? type).toLowerCase().replaceAll('_', ' ') }
function displayLabel(type: string) { return editableBlockTypes.find((entry) => entry.type === type)?.label ?? type.replaceAll('_', ' ') }
function openPicker(event: Event) { pickerTrigger.value = event.currentTarget instanceof HTMLElement ? event.currentTarget : undefined; pickerOpen.value = true }
function preview() { commitEditing(false); if (dirty.value) { if (previewWarning.value?.showModal) previewWarning.value.showModal(); else previewWarning.value?.setAttribute('open', ''); return }; emit('preview') }
function closePreviewWarning() { if (previewWarning.value && typeof previewWarning.value.close === 'function' && previewWarning.value.open) previewWarning.value.close(); else previewWarning.value?.removeAttribute('open') }
function onPickerClosed(open: boolean) { if (!open) void nextTick(() => pickerTrigger.value?.focus()) }
function closePicker(restoreFocus = true) { pickerOpen.value = false; if (restoreFocus) void nextTick(() => pickerTrigger.value?.focus()) }
function addBlock(blockType: EditableBlockType) {
  if (saving.value || document.value.blocks.length >= contentEditorLimits.blocks) return
  const block = createContentBlock(blockType, document.value.blocks.map((entry) => entry.key))
  const next = { ...document.value, blocks: [...document.value.blocks, block] }
  if (new TextEncoder().encode(JSON.stringify(next)).length > contentEditorLimits.bytes) { formError.value = 'Lesson content must fit within 1 MiB.'; return }
  document.value = next
  message.value = undefined
  closePicker(false)
  const index = next.blocks.length - 1
  void nextTick(() => {
    if (blockType === 'DIVIDER') globalThis.document.getElementById(`authoring-content-block-${block.key}`)?.focus()
    else startEditing(index)
  })
}
function blockSummary(block: CanonicalBlock): string { const p: any = block.payload; if (block.type === 'TEXT') return richTextPlainText(p.content).slice(0, 140) || 'Written content'; if (block.type === 'HEADING') return p.content.map((i: any) => i.text).join(' '); if (block.type === 'IMAGE') return p.caption || p.altText || 'Image asset'; if (block.type === 'VIDEO' || block.type === 'AUDIO') return p.title; if (block.type === 'DOWNLOAD') return p.label; if (block.type === 'CODE') return [p.language, p.code.split('\n')[0]].filter(Boolean).join(' · '); if (block.type === 'QUOTE') return p.text.slice(0,140); if (block.type === 'CALLOUT') return p.title || 'Callout'; if (block.type === 'KNOWLEDGE_CHECK') return 'Inline learner assessment'; if (block.type === 'PLUGIN_WIDGET') return 'Course widget'; return '' }
function contentIcon(type: string): BtgIconName {
  return ({ TEXT: 'text', HEADING: 'text', IMAGE: 'image', VIDEO: 'content', AUDIO: 'content', DOWNLOAD: 'document', CODE: 'document', CALLOUT: 'callout', QUOTE: 'document', DIVIDER: 'divider', KNOWLEDGE_CHECK: 'objective', PLUGIN_WIDGET: 'settings' } as Record<string, BtgIconName>)[type] ?? 'content'
}
function imageURL(block: CanonicalBlock): string | undefined {
  return block.type === 'IMAGE' ? draftAssetURL({ draftID: props.draftId }, block.payload.asset) : undefined
}
function copyBlock(block: CanonicalBlock) { return JSON.parse(JSON.stringify(block)) as CanonicalBlock }
function isEditing(block: CanonicalBlock) { return editingKey.value === block.key }
function focusEditor(key: string) {
  const region = globalThis.document.getElementById(`authoring-content-editor-${key}`)
  region?.querySelector<HTMLElement>('[contenteditable="true"], input, textarea, select, button')?.focus()
}
function focusEditControl(key: string) { globalThis.document.getElementById(`authoring-content-edit-${key}`)?.focus() }
function finishEditing(restoreFocus: boolean) {
  const key = editingKey.value
  editingKey.value = undefined
  editingBlock.value = undefined
  editingOriginal.value = undefined
  if (restoreFocus && key) void nextTick(() => focusEditControl(key))
}
function commitEditing(restoreFocus = true) { if (editingKey.value) finishEditing(restoreFocus) }
function startEditing(index: number) {
  if (saving.value) return
  const block = document.value.blocks[index]
  if (!block || block.type === 'DIVIDER') return
  if (isEditing(block)) return
  commitEditing(false)
  editingKey.value = block.key
  editingOriginal.value = copyBlock(block)
  editingBlock.value = copyBlock(block)
  closeActions()
  void nextTick(() => focusEditor(block.key))
}
function updateEditing(block: CanonicalBlock) {
  if (!editingKey.value || editingKey.value !== block.key) return
  editingBlock.value = copyBlock(block)
  updateBlock(block.key, block)
}
function doneEditing() { commitEditing(true) }
function cancelEditing() {
  if (editingKey.value && editingOriginal.value) updateBlock(editingKey.value, editingOriginal.value)
  finishEditing(true)
}
function toggleActions(index: number) { actionsIndex.value = actionsIndex.value === index ? undefined : index }
function closeActions() { actionsIndex.value = undefined }
function closeActionsOnOutsidePointer(event: PointerEvent) {
  if (actionsIndex.value === undefined) return
  const target = event.target
  if (target instanceof Element && target.closest('.authoring-content__menu')) return
  closeActions()
}
function updateBlock(key: string, block: CanonicalBlock) {
  if (saving.value) return
  const index = document.value.blocks.findIndex((current) => current.key === key && current.type === block.type)
  if (index < 0 || block.key !== key) return
  const blocks = [...document.value.blocks]
  blocks[index] = block
  document.value = { ...document.value, blocks }
  message.value = undefined
}
function discardChanges() {
  if (saving.value || reloading.value) return
  document.value = copyContent(savedContent.value)
  baseline.value = contentFingerprint(savedContent.value)
  conflict.value = false
  formError.value = undefined
  finishEditing(false)
  closeActions()
  message.value = 'Unsaved content changes discarded.'
}
function focusReorderHandle(key: string) { globalThis.document.getElementById(`authoring-content-reorder-${key}`)?.focus() }
function moveBlock(fromIndex: number, toIndex: number, focus = false): boolean {
  if (saving.value || fromIndex < 0 || toIndex < 0 || fromIndex >= document.value.blocks.length || toIndex >= document.value.blocks.length || fromIndex === toIndex) return false
  commitEditing(false)
  const blocks = [...document.value.blocks]
  const [block] = blocks.splice(fromIndex, 1)
  if (!block) return false
  blocks.splice(toIndex, 0, block)
  document.value = { ...document.value, blocks }
  closeActions()
  message.value = undefined
  reorderAnnouncement.value = `${displayLabel(block.type)} block moved to position ${toIndex + 1} of ${blocks.length}.`
  if (focus) void nextTick(() => focusReorderHandle(block.key))
  return true
}
function move(index: number, direction: -1 | 1) {
  moveBlock(index, index + direction)
}
function clearPointerReorder() {
  globalThis.document.removeEventListener('pointermove', updatePointerReorder)
  globalThis.document.removeEventListener('pointerup', finishPointerReorder)
  globalThis.document.removeEventListener('pointercancel', cancelPointerReorder)
  dragPointerID = undefined
  dragSourceIndex = -1
  draggingKey.value = undefined
  insertionIndex.value = undefined
}
function startPointerReorder(event: PointerEvent, index: number) {
  if (saving.value || event.button !== 0) return
  const block = document.value.blocks[index]
  if (!block) return
  event.preventDefault()
  commitEditing(false)
  closeActions()
  keyboardReorderKey.value = undefined
  dragPointerID = event.pointerId
  dragSourceIndex = index
  draggingKey.value = block.key
  insertionIndex.value = index
  globalThis.document.addEventListener('pointermove', updatePointerReorder)
  globalThis.document.addEventListener('pointerup', finishPointerReorder)
  globalThis.document.addEventListener('pointercancel', cancelPointerReorder)
}
function updatePointerReorder(event: PointerEvent) {
  if (event.pointerId !== dragPointerID || draggingKey.value === undefined) return
  const row = globalThis.document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>('[data-content-block-key]')
  if (row?.dataset.contentBlockKey) {
    const target = document.value.blocks.findIndex((block) => block.key === row.dataset.contentBlockKey)
    if (target >= 0) {
      const bounds = row.getBoundingClientRect()
      insertionIndex.value = event.clientY < bounds.top + bounds.height / 2 ? target : target + 1
    }
  }
  const edge = 3 * 16
  if (event.clientY < edge) globalThis.scrollBy({ top: -16, behavior: 'auto' })
  else if (event.clientY > globalThis.innerHeight - edge) globalThis.scrollBy({ top: 16, behavior: 'auto' })
}
function finishPointerReorder(event: PointerEvent) {
  if (event.pointerId !== dragPointerID) return
  const key = draggingKey.value
  const source = dragSourceIndex
  const insertion = insertionIndex.value ?? source
  clearPointerReorder()
  if (!key || source < 0) return
  const destination = insertion > source ? insertion - 1 : insertion
  moveBlock(source, destination, true)
}
function cancelPointerReorder(event: PointerEvent) {
  if (event.pointerId !== dragPointerID) return
  const key = draggingKey.value
  clearPointerReorder()
  if (key) void nextTick(() => focusReorderHandle(key))
}
function keyboardReorder(event: KeyboardEvent, index: number) {
  const block = document.value.blocks[index]
  if (!block || saving.value) return
  if (event.key === ' ' || event.key === 'Enter') {
    event.preventDefault()
    keyboardReorderKey.value = keyboardReorderKey.value === block.key ? undefined : block.key
    reorderAnnouncement.value = keyboardReorderKey.value === block.key
      ? `Reordering ${displayLabel(block.type)} block. Use the arrow keys to move it, or Escape to finish.`
      : `Finished reordering ${displayLabel(block.type)} block.`
    return
  }
  if (event.key === 'Escape' && keyboardReorderKey.value === block.key) {
    event.preventDefault()
    keyboardReorderKey.value = undefined
    reorderAnnouncement.value = `Finished reordering ${displayLabel(block.type)} block.`
    return
  }
  if (keyboardReorderKey.value !== block.key || (event.key !== 'ArrowUp' && event.key !== 'ArrowDown')) return
  event.preventDefault()
  const current = document.value.blocks.findIndex((entry) => entry.key === block.key)
  moveBlock(current, current + (event.key === 'ArrowUp' ? -1 : 1), true)
}
function removeBlock(index: number) {
  if (saving.value) return
  commitEditing(false)
  const restoreFocus = preserveFocusAfterRemoval(() => globalThis.document.querySelector<HTMLElement>('.authoring-content__add button'))
  document.value.blocks.splice(index, 1)
  closeActions()
  void restoreFocus()
  message.value = undefined
}

async function save() {
  const isCurrent = captureScope()
  commitEditing(false)
  if (saving.value || reloading.value || conflict.value || !supported.value || !dirty.value) return
  formError.value = contentEditorError(document.value)
  if (formError.value) return
  const generation = requestVersion
  saving.value = true
  message.value = undefined
  try {
    const result = await replaceAuthoringLessonContent(props.draftId, props.lesson.id, { expectedLessonRevision: props.lesson.revision, content: copyContent(document.value) })
    if (!isCurrent()) return
    if (!active || generation !== requestVersion) return
    initialize(result.content)
    emit('saved', result)
    message.value = 'Lesson content saved.'
  } catch (error) {
    if (!isCurrent()) return
    if (!active || generation !== requestVersion) return
    if (error instanceof APIProblemError && error.status === 404) { emit('unavailable'); return }
    if (error instanceof APIProblemError && error.status === 409) { conflict.value = true; return }
    formError.value = error instanceof APIProblemError && error.status === 400 ? 'We couldn’t save this content. Check the block fields and try again.'
      : error instanceof APIProblemError && error.status === 413 ? 'This content is too large to save. Shorten it and try again.'
        : 'We couldn’t save Lesson content right now. Please try again.'
  } finally { if (generation === requestVersion) saving.value = false }
}
async function reloadLatest() {
  const isCurrent = captureScope()
  if (reloading.value || saving.value) return
  const generation = requestVersion
  reloading.value = true
  formError.value = undefined
  try {
    const latest = await getAuthoringLesson(props.draftId, props.lesson.id)
    if (!isCurrent()) return
    if (!active || generation !== requestVersion) return
    initialize(latest.content)
    emit('replaceLesson', latest)
  } catch (error) {
    if (!isCurrent()) return
    if (!active || generation !== requestVersion) return
    if (error instanceof APIProblemError && error.status === 404) emit('unavailable')
    else formError.value = 'We couldn’t reload Lesson content right now. Please try again.'
  } finally { if (generation === requestVersion) reloading.value = false }
}
</script>

<template>
  <section class="authoring-content" aria-label="Lesson content editor">
    <p v-if="!supported" role="alert">This content format cannot be edited safely by this version of the Academy.</p>
    <form v-else :aria-busy="saving" novalidate @submit.prevent="save">
      <p v-if="formError" class="authoring-content__error" role="alert">{{ formError }}</p>
      <p v-if="message" class="authoring-content__status" role="status">{{ message }}</p>
      <p class="sr-only" role="status" aria-live="polite">{{ reorderAnnouncement }}</p>
      <div v-if="conflict" class="authoring-content__conflict" role="status">
        <p>Lesson content changed elsewhere. Your unsaved blocks are still here. Reload the latest content before saving again.</p>
        <BtgButton variant="secondary" :disabled="reloading" @click="reloadLatest">{{ reloading ? 'Reloading…' : 'Reload latest content' }}</BtgButton>
      </div>
      <fieldset class="authoring-content__controls" :disabled="saving || reloading">
        <legend class="sr-only">Content blocks</legend>
        <BtgToolbar v-if="document.blocks.length" class="authoring-content__toolbar"><template #actions><BtgButton leading-icon="plus" :disabled="document.blocks.length >= contentEditorLimits.blocks" @click="openPicker">+ Add content</BtgButton><BtgButton variant="secondary" leading-icon="content" @click="preview">Preview</BtgButton></template></BtgToolbar>
        <p v-if="document.blocks.length >= contentEditorLimits.blocks">The Lesson has reached the 200-block limit.</p>
        <p id="authoring-content-reorder-help" class="sr-only">Drag this handle with a mouse or touch to reorder the block. Press Space or Enter, then use the arrow keys to reorder with the keyboard. Move up and Move down are also available in Actions.</p>
        <ol v-if="document.blocks.length" class="authoring-content__blocks" aria-label="Lesson content blocks">
          <li v-for="(block, index) in document.blocks" :id="`authoring-content-block-${block.key}`" :key="block.key" :data-content-block-key="block.key" :class="['authoring-content__row', { 'is-expanded': isEditing(block), 'is-menu-open': actionsIndex === index, 'is-dragging': draggingKey === block.key, 'is-drop-before': draggingKey !== block.key && insertionIndex === index, 'is-drop-after': draggingKey !== block.key && insertionIndex === document.blocks.length && index === document.blocks.length - 1 }]" tabindex="-1">
            <div class="authoring-content__row-header">
              <BtgDragHandle :id="`authoring-content-reorder-${block.key}`" :label="`Reorder block ${index + 1}, ${label(block.type)}`" :aria-pressed="keyboardReorderKey === block.key" aria-describedby="authoring-content-reorder-help" @pointerdown="startPointerReorder($event, index)" @keydown="keyboardReorder($event, index)" />
              <span class="authoring-content__number">{{ String(index + 1).padStart(2, '0') }}</span>
              <span class="authoring-content__type-icon"><BtgIcon :name="contentIcon(block.type)" decorative /></span>
              <div class="authoring-content__summary">
                <img v-if="imageURL(block)" class="authoring-content__image-preview" :src="imageURL(block)" alt="" />
                <div>
                  <p>{{ label(block.type) }}</p>
                  <strong v-if="block.type !== 'DIVIDER'">{{ blockSummary(block) }}</strong>
                  <hr v-else />
                </div>
              </div>
              <div class="authoring-content__row-actions">
                <template v-if="block.type !== 'DIVIDER'">
                  <BtgButton v-if="!isEditing(block)" :id="`authoring-content-edit-${block.key}`" variant="secondary" :aria-controls="`authoring-content-editor-${block.key}`" :aria-expanded="false" :aria-label="`Edit block ${index + 1}, ${label(block.type)}`" @click="startEditing(index)">Edit</BtgButton>
                  <template v-else>
                    <BtgButton variant="secondary" :aria-label="`Cancel changes for block ${index + 1}, ${label(block.type)}`" @click="cancelEditing">Cancel</BtgButton>
                    <BtgButton :id="`authoring-content-edit-${block.key}`" :aria-controls="`authoring-content-editor-${block.key}`" :aria-expanded="true" :aria-label="`Done editing block ${index + 1}, ${label(block.type)}`" @click="doneEditing">Done</BtgButton>
                  </template>
                </template>
                <div class="authoring-content__menu">
                  <BtgIconButton icon="overflow" :aria-expanded="actionsIndex === index" :label="`Actions for block ${index + 1}, ${label(block.type)}`" @click="toggleActions(index)" />
                  <div v-if="actionsIndex === index" role="menu" class="authoring-content__menu-items" @keydown.esc="closeActions">
                    <BtgButton role="menuitem" variant="secondary" :disabled="index === 0" @click="move(index, -1)">Move up</BtgButton>
                    <BtgButton role="menuitem" variant="secondary" :disabled="index === document.blocks.length - 1" @click="move(index, 1)">Move down</BtgButton>
                    <BtgButton role="menuitem" variant="destructive" @click="removeBlock(index)">Remove</BtgButton>
                  </div>
                </div>
              </div>
            </div>
            <section v-if="isEditing(block) && editingBlock" :id="`authoring-content-editor-${block.key}`" class="authoring-content__inline-editor" :aria-label="`Editing block ${index + 1}, ${label(block.type)}`">
              <AuthoringContentBlock :block="editingBlock" :position="index + 1" :draft-id="draftId" :lesson-id="lesson.id" :course-widgets="courseWidgets" @update="updateEditing" @unavailable="emit('unavailable')" />
            </section>
          </li>
        </ol>
        <div v-else class="authoring-content__empty"><p><strong>No lesson content yet.</strong></p><p>Add text, media, code, callouts, and other learning material.</p><BtgButton @click="openPicker">+ Add content</BtgButton><BtgButton variant="secondary" @click="preview">Preview</BtgButton></div>
      </fieldset>
      <AuthoringDirtyActionBar :show="dirty" :busy="saving" busy-label="Saving content…" save-label="Save content" :disabled="saving || reloading" :save-disabled="conflict" @discard="discardChanges" />
    </form>
  </section>
  <BtgDialog v-model:open="pickerOpen" class="authoring-content__picker" :title="t('authoring.content.picker.title')" :description="t('authoring.content.picker.description')" :dismiss-label="t('authoring.content.picker.close')" :show-trigger="false" @update:open="onPickerClosed">
      <section v-for="group in pickerGroups" :key="group.id" class="authoring-content__picker-group" :aria-labelledby="`authoring-content-picker-${group.id}`">
        <h4 :id="`authoring-content-picker-${group.id}`">{{ t(`authoring.content.picker.groups.${group.id}`) }}</h4>
        <div class="authoring-content__picker-options">
          <BtgButton v-for="type in group.types" :key="type" type="button" variant="secondary" :aria-label="t(`authoring.content.picker.types.${type}.label`)" :aria-describedby="`authoring-content-picker-${type}`" @click="addBlock(type)">
            <BtgIcon :name="contentIcon(type)" decorative />
            <span><strong>{{ t(`authoring.content.picker.types.${type}.label`) }}</strong><small :id="`authoring-content-picker-${type}`">{{ t(`authoring.content.picker.types.${type}.description`) }}</small></span>
          </BtgButton>
        </div>
      </section>
  </BtgDialog>
  <dialog ref="previewWarning" class="authoring-content__picker authoring-content__preview-warning" aria-labelledby="authoring-preview-warning-title"><div><h3 id="authoring-preview-warning-title">You have unsaved changes</h3><p>Preview will show the last saved version.</p><footer><BtgButton variant="secondary" @click="closePreviewWarning">Cancel</BtgButton><BtgButton variant="secondary" @click="closePreviewWarning(); emit('preview')">Preview saved version</BtgButton><BtgButton @click="save().then(() => { if (!formError) { closePreviewWarning(); emit('preview') } })">Save and preview</BtgButton></footer></div></dialog>
</template>
