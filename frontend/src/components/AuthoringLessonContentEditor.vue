<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { APIProblemError } from '../api/client'
import { preserveFocusAfterRemoval } from '../authoring/focus'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { getAuthoringLesson, listAuthoringCourseWidgets, replaceAuthoringLessonContent, type AuthoringCourseWidget, type AuthoringLessonContent, type AuthoringLessonContentMutation, type AuthoringLessonDetail } from '../authoring/authoring'
import { contentEditorError, contentEditorLimits, contentFingerprint, copyContent, createContentBlock, editableBlockTypes, type CanonicalBlock, type EditableBlockType } from '../authoring/contentEditor'
import { decodeLessonContent } from '../lesson/content'
import AuthoringContentBlock from './AuthoringContentBlock.vue'
import BtgButton from './BtgButton.vue'

const props = defineProps<{ draftId: string; lesson: AuthoringLessonDetail }>()
const emit = defineEmits<{ saved: [result: AuthoringLessonContentMutation]; replaceLesson: [lesson: AuthoringLessonDetail]; preview: []; unavailable: [] }>()
const document = ref<AuthoringLessonContent>(copyContent(props.lesson.content))
const baseline = ref(contentFingerprint(props.lesson.content))
const supported = ref(false)
const picker = ref<HTMLDialogElement>()
const previewWarning = ref<HTMLDialogElement>()
const blockEditor = ref<HTMLDialogElement>()
const editingIndex = ref<number>()
const editingBlock = ref<CanonicalBlock>()
const blockEditorTrigger = ref<HTMLElement>()
const actionsIndex = ref<number>()
const pickerTrigger = ref<HTMLElement>()
const saving = ref(false)
const reloading = ref(false)
const conflict = ref(false)
const formError = ref<string>()
const message = ref<string>()
const courseWidgets = ref<AuthoringCourseWidget[]>([])
const dirty = computed(() => contentFingerprint(document.value) !== baseline.value)
const pickerGroups = [
  { id: 'content', label: 'Content', types: ['TEXT', 'HEADING', 'IMAGE', 'VIDEO', 'AUDIO', 'DOWNLOAD', 'CODE', 'CALLOUT', 'QUOTE', 'DIVIDER'] as EditableBlockType[] },
  { id: 'interactive', label: 'Interactive', types: ['KNOWLEDGE_CHECK', 'PLUGIN_WIDGET'] as EditableBlockType[] },
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
  baseline.value = contentFingerprint(content)
  const decoded = decodeLessonContent(content)
  supported.value = decoded.kind === 'ready' && decoded.blocks.every((block) => block.type !== 'UNSUPPORTED')
  conflict.value = false
  formError.value = undefined
  message.value = undefined
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
onBeforeUnmount(() => { active = false })

function label(type: string) { return (editableBlockTypes.find((entry) => entry.type === type)?.label ?? type).toLowerCase().replaceAll('_', ' ') }
function openPicker(event: Event) { pickerTrigger.value = event.currentTarget instanceof HTMLElement ? event.currentTarget : undefined; if (picker.value?.showModal) picker.value.showModal(); else picker.value?.setAttribute('open', '') }
function preview() { if (dirty.value) { if (previewWarning.value?.showModal) previewWarning.value.showModal(); else previewWarning.value?.setAttribute('open', ''); return }; emit('preview') }
function closePreviewWarning() { if (previewWarning.value && typeof previewWarning.value.close === 'function' && previewWarning.value.open) previewWarning.value.close(); else previewWarning.value?.removeAttribute('open') }
function onPickerClosed() { closePicker(true) }
function closePicker(restoreFocus = true) { const dialog = picker.value; if (dialog && typeof dialog.close === 'function' && dialog.open) dialog.close(); else dialog?.removeAttribute('open'); if (restoreFocus) void nextTick(() => pickerTrigger.value?.focus()) }
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
    else openBlockEditor(index)
  })
}
function blockSummary(block: CanonicalBlock): string { const p: any = block.payload; if (block.type === 'TEXT') return p.content.nodes.flatMap((n: any) => n.content ?? []).filter((i: any) => i.type === 'text').map((i: any) => i.text).join(' ').slice(0, 140) || 'Written content'; if (block.type === 'HEADING') return p.content.map((i: any) => i.text).join(' '); if (block.type === 'IMAGE') return p.caption || p.altText || 'Image asset'; if (block.type === 'VIDEO' || block.type === 'AUDIO') return p.title; if (block.type === 'DOWNLOAD') return p.label; if (block.type === 'CODE') return [p.language, p.code.split('\n')[0]].filter(Boolean).join(' · '); if (block.type === 'QUOTE') return p.text.slice(0,140); if (block.type === 'CALLOUT') return p.title || 'Callout'; if (block.type === 'KNOWLEDGE_CHECK') return 'Inline learner assessment'; if (block.type === 'PLUGIN_WIDGET') return 'Course widget'; return '' }
function openBlockEditor(index: number, event?: Event) { blockEditorTrigger.value = event?.currentTarget instanceof HTMLElement ? event.currentTarget : undefined; editingIndex.value=index; editingBlock.value=JSON.parse(JSON.stringify(document.value.blocks[index])); if (blockEditor.value?.showModal) blockEditor.value.showModal(); else blockEditor.value?.setAttribute('open','') }
function onBlockEditorClosed() { editingIndex.value = undefined; editingBlock.value = undefined; void nextTick(() => blockEditorTrigger.value?.focus()) }
function closeBlockEditor() { if (blockEditor.value && typeof blockEditor.value.close === 'function' && blockEditor.value.open) blockEditor.value.close(); else { blockEditor.value?.removeAttribute('open'); onBlockEditorClosed() } }
function applyBlockEditor() { if (editingIndex.value === undefined || !editingBlock.value) return; updateBlock(document.value.blocks[editingIndex.value]!.key, editingBlock.value); closeBlockEditor() }
function toggleActions(index: number) { actionsIndex.value = actionsIndex.value === index ? undefined : index }
function closeActions() { actionsIndex.value = undefined }
function updateBlock(key: string, block: CanonicalBlock) {
  if (saving.value) return
  const index = document.value.blocks.findIndex((current) => current.key === key && current.type === block.type)
  if (index < 0 || block.key !== key) return
  const blocks = [...document.value.blocks]
  blocks[index] = block
  document.value = { ...document.value, blocks }
  message.value = undefined
}
function move(index: number, direction: -1 | 1) {
  if (saving.value) return
  const destination = index + direction
  if (destination < 0 || destination >= document.value.blocks.length) return
  const blocks = [...document.value.blocks]
  const [block] = blocks.splice(index, 1)
  blocks.splice(destination, 0, block!)
  document.value = { ...document.value, blocks }
  closeActions()
  message.value = undefined
}
function removeBlock(index: number) {
  if (saving.value) return
  const restoreFocus = preserveFocusAfterRemoval(() => globalThis.document.querySelector<HTMLElement>('.authoring-content__add button'))
  document.value.blocks.splice(index, 1)
  closeActions()
  void restoreFocus()
  message.value = undefined
}

async function save() {
  const isCurrent = captureScope()
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
  <section class="authoring-content" aria-labelledby="authoring-content-title">
    <h3 id="authoring-content-title">Lesson content</h3>
    <p>Build the material learners will work through in this lesson.</p>
    <p v-if="!supported" role="alert">This content format cannot be edited safely by this version of the Academy.</p>
    <form v-else :aria-busy="saving" novalidate @submit.prevent="save">
      <p v-if="formError" class="authoring-content__error" role="alert">{{ formError }}</p>
      <p v-if="message" class="authoring-content__status" role="status">{{ message }}</p>
      <div v-if="conflict" class="authoring-content__conflict" role="status">
        <p>Lesson content changed elsewhere. Your unsaved blocks are still here. Reload the latest content before saving again.</p>
        <BtgButton variant="secondary" :disabled="reloading" @click="reloadLatest">{{ reloading ? 'Reloading…' : 'Reload latest content' }}</BtgButton>
      </div>
      <fieldset class="authoring-content__controls" :disabled="saving || reloading">
        <legend class="sr-only">Content blocks</legend>
        <div v-if="document.blocks.length" class="authoring-content__add"><BtgButton variant="secondary" :disabled="document.blocks.length >= contentEditorLimits.blocks" @click="openPicker">+ Add content</BtgButton><BtgButton variant="secondary" @click="preview">Preview</BtgButton></div>
        <p v-if="document.blocks.length >= contentEditorLimits.blocks">The Lesson has reached the 200-block limit.</p>
        <ol v-if="document.blocks.length" class="authoring-content__blocks" aria-label="Lesson content blocks">
          <li v-for="(block, index) in document.blocks" :id="`authoring-content-block-${block.key}`" :key="block.key" class="authoring-content__row" tabindex="-1">
            <span class="authoring-content__number">{{ String(index + 1).padStart(2, '0') }}</span>
            <div class="authoring-content__summary">
              <p>{{ label(block.type) }}</p>
              <strong v-if="block.type !== 'DIVIDER'">{{ blockSummary(block) }}</strong>
              <hr v-else />
            </div>
            <div class="authoring-content__row-actions">
              <BtgButton v-if="block.type !== 'DIVIDER'" variant="secondary" :aria-label="`Edit block ${index + 1}, ${label(block.type)}`" @click="openBlockEditor(index, $event)">Edit</BtgButton>
              <div class="authoring-content__menu">
                <BtgButton variant="secondary" :aria-expanded="actionsIndex === index" :aria-label="`Actions for block ${index + 1}, ${label(block.type)}`" @click="toggleActions(index)">Actions</BtgButton>
                <div v-if="actionsIndex === index" role="menu" class="authoring-content__menu-items" @keydown.esc="closeActions">
                  <BtgButton role="menuitem" variant="secondary" :disabled="index === 0" @click="move(index, -1)">Move up</BtgButton>
                  <BtgButton role="menuitem" variant="secondary" :disabled="index === document.blocks.length - 1" @click="move(index, 1)">Move down</BtgButton>
                  <BtgButton role="menuitem" variant="destructive" @click="removeBlock(index)">Remove</BtgButton>
                </div>
              </div>
            </div>
          </li>
        </ol>
        <div v-else class="authoring-content__empty"><p><strong>No lesson content yet.</strong></p><p>Add text, media, code, callouts, and other learning material.</p><BtgButton @click="openPicker">+ Add content</BtgButton><BtgButton variant="secondary" @click="preview">Preview</BtgButton></div>
      </fieldset>
      <p v-if="dirty" role="status">You have unsaved content changes.</p>
      <div class="authoring-content__actions"><BtgButton type="submit" :disabled="!dirty || saving || reloading || conflict">{{ saving ? 'Saving content…' : 'Save Lesson content' }}</BtgButton></div>
    </form>
  </section>
  <dialog ref="picker" class="authoring-content__picker" aria-labelledby="authoring-content-picker-title" @close="onPickerClosed">
    <div>
      <header><h3 id="authoring-content-picker-title">Add content</h3><p>Choose the kind of learning material to add.</p></header>
      <section v-for="group in pickerGroups" :key="group.id" class="authoring-content__picker-group" :aria-labelledby="`authoring-content-picker-${group.id}`">
        <h4 :id="`authoring-content-picker-${group.id}`">{{ group.label }}</h4>
        <div class="authoring-content__picker-options">
          <button v-for="type in group.types" :key="type" type="button" :aria-label="pickerOptions.get(type)?.label" :aria-describedby="`authoring-content-picker-${type}`" @click="addBlock(type)">
            <svg aria-hidden="true" viewBox="0 0 24 24"><path :d="pickerOptions.get(type)?.icon" /></svg>
            <span><strong>{{ pickerOptions.get(type)?.label }}</strong><small :id="`authoring-content-picker-${type}`">{{ pickerOptions.get(type)?.description }}</small></span>
          </button>
        </div>
      </section>
      <footer><BtgButton variant="secondary" @click="closePicker">Cancel</BtgButton></footer>
    </div>
  </dialog>
  <dialog ref="blockEditor" class="authoring-content__picker authoring-content__block-editor" aria-labelledby="authoring-block-editor-title" @close="onBlockEditorClosed"><div><h3 id="authoring-block-editor-title">Edit {{ editingBlock ? label(editingBlock.type) : 'content' }} block</h3><AuthoringContentBlock v-if="editingBlock" :block="editingBlock" :position="(editingIndex ?? 0) + 1" :draft-id="draftId" :lesson-id="lesson.id" :course-widgets="courseWidgets" @update="editingBlock = $event" @unavailable="emit('unavailable')" /><footer><BtgButton variant="secondary" @click="closeBlockEditor">Cancel</BtgButton><BtgButton @click="applyBlockEditor">Apply</BtgButton></footer></div></dialog>
  <dialog ref="previewWarning" class="authoring-content__picker authoring-content__preview-warning" aria-labelledby="authoring-preview-warning-title"><div><h3 id="authoring-preview-warning-title">You have unsaved changes</h3><p>Preview will show the last saved version.</p><footer><BtgButton variant="secondary" @click="closePreviewWarning">Cancel</BtgButton><BtgButton variant="secondary" @click="closePreviewWarning(); emit('preview')">Preview saved version</BtgButton><BtgButton @click="save().then(() => { if (!formError) { closePreviewWarning(); emit('preview') } })">Save and preview</BtgButton></footer></div></dialog>
</template>
