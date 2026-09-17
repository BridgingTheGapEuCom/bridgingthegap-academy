<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { APIProblemError } from '../api/client'
import { preserveFocusAfterRemoval } from '../authoring/focus'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { getAuthoringLesson, replaceAuthoringLessonContent, type AuthoringLessonContent, type AuthoringLessonContentMutation, type AuthoringLessonDetail } from '../authoring/authoring'
import { contentEditorError, contentEditorLimits, contentFingerprint, copyContent, createContentBlock, editableBlockTypes, type CanonicalBlock, type EditableBlockType } from '../authoring/contentEditor'
import { decodeLessonContent } from '../lesson/content'
import AuthoringContentBlock from './AuthoringContentBlock.vue'
import BtgButton from './BtgButton.vue'
import BtgFormField from './BtgFormField.vue'

const props = defineProps<{ draftId: string; lesson: AuthoringLessonDetail }>()
const emit = defineEmits<{ saved: [result: AuthoringLessonContentMutation]; replaceLesson: [lesson: AuthoringLessonDetail]; unavailable: [] }>()
const document = ref<AuthoringLessonContent>(copyContent(props.lesson.content))
const baseline = ref(contentFingerprint(props.lesson.content))
const supported = ref(false)
const blockType = ref<EditableBlockType>('TEXT')
const saving = ref(false)
const reloading = ref(false)
const conflict = ref(false)
const formError = ref<string>()
const message = ref<string>()
const dirty = computed(() => contentFingerprint(document.value) !== baseline.value)
let loadedLessonID = ''
let requestVersion = 0
let active = true


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
  } else if (contentFingerprint(props.lesson.content) !== baseline.value) {
    if (dirty.value) conflict.value = true
    else initialize(props.lesson.content)
  }
}, { immediate: true })
onBeforeUnmount(() => { active = false })

function label(type: string) { return (editableBlockTypes.find((entry) => entry.type === type)?.label ?? type).toLowerCase().replaceAll('_', ' ') }
function addBlock() {
  if (saving.value || document.value.blocks.length >= contentEditorLimits.blocks) return
  const next = { ...document.value, blocks: [...document.value.blocks, createContentBlock(blockType.value, document.value.blocks.map((block) => block.key))] }
  if (new TextEncoder().encode(JSON.stringify(next)).length > contentEditorLimits.bytes) { formError.value = 'Lesson content must fit within 1 MiB.'; return }
  document.value = next
  message.value = undefined
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
function move(index: number, direction: -1 | 1) {
  if (saving.value) return
  const destination = index + direction
  if (destination < 0 || destination >= document.value.blocks.length) return
  const blocks = [...document.value.blocks]
  const [block] = blocks.splice(index, 1)
  blocks.splice(destination, 0, block!)
  document.value = { ...document.value, blocks }
  message.value = undefined
}
function removeBlock(index: number) {
  if (saving.value) return
  const restoreFocus = preserveFocusAfterRemoval(() => globalThis.document.querySelector<HTMLElement>('.authoring-content__add button'))
  document.value.blocks.splice(index, 1)
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
    <p>Edit the ordered semantic blocks, then save the complete Lesson content when you are ready.</p>
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
        <div class="authoring-content__add">
          <BtgFormField label="Block type" v-slot="{ controlId }"><select :id="controlId" v-model="blockType"><option v-for="entry in editableBlockTypes" :key="entry.type" :value="entry.type">{{ entry.label }}</option></select></BtgFormField>
          <BtgButton variant="secondary" :disabled="document.blocks.length >= contentEditorLimits.blocks" @click="addBlock">Add block</BtgButton>
        </div>
        <p v-if="document.blocks.length >= contentEditorLimits.blocks">The Lesson has reached the 200-block limit.</p>
        <ol v-if="document.blocks.length" class="authoring-content__blocks" aria-label="Lesson content blocks">
          <li v-for="(block, index) in document.blocks" :key="block.key">
            <fieldset class="authoring-content__block">
              <legend>Block {{ index + 1 }} · {{ label(block.type) }}</legend>
              <p class="authoring-content__key">Stable key: <code>{{ block.key }}</code></p>
              <AuthoringContentBlock :block="block" :position="index + 1" :draft-id="draftId" :lesson-id="lesson.id" @update="updateBlock(block.key, $event)" @unavailable="emit('unavailable')" />
              <div class="authoring-content__actions">
                <BtgButton variant="secondary" :disabled="index === 0" :aria-label="`Move block ${index + 1} ${label(block.type)} up`" @click="move(index, -1)">Move up</BtgButton>
                <BtgButton variant="secondary" :disabled="index === document.blocks.length - 1" :aria-label="`Move block ${index + 1} ${label(block.type)} down`" @click="move(index, 1)">Move down</BtgButton>
                <BtgButton variant="secondary" :aria-label="`Remove block ${index + 1} ${label(block.type)}`" @click="removeBlock(index)">Remove block</BtgButton>
              </div>
            </fieldset>
          </li>
        </ol>
        <p v-else>No content blocks yet. Add a block to begin.</p>
      </fieldset>
      <p v-if="dirty" role="status">You have unsaved content changes.</p>
      <div class="authoring-content__actions"><BtgButton type="submit" :disabled="!dirty || saving || reloading || conflict">{{ saving ? 'Saving content…' : 'Save Lesson content' }}</BtgButton></div>
    </form>
  </section>
</template>
