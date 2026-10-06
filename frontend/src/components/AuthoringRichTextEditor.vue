<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Link from '@tiptap/extension-link'
import StarterKit from '@tiptap/starter-kit'
import { Editor, EditorContent } from '@tiptap/vue-3'
import type { components } from '../api/generated'
import type { RichText } from '../lesson/content'
import { safePublishedURL } from '../lesson/content'
import { editorDocumentToRichText, richTextToEditorDocument } from '../authoring/richText'
import BtgIconButton from './BtgIconButton.vue'
import BtgButton from './BtgButton.vue'
import BtgTextInput from './BtgTextInput.vue'

type APIRichText = components['schemas']['RichText']
const props = defineProps<{ content: APIRichText; label: string; codeBlocks?: boolean }>()
const emit = defineEmits<{ 'update:content': [content: APIRichText] }>()
const { t } = useI18n()
const linkOpen = ref(false)
const linkURL = ref('')
const linkError = ref<string>()
const linkTrigger = ref<HTMLElement>()
const editorRevision = ref(0)
let applyingExternalValue = false

const editor = new Editor({
  content: richTextToEditorDocument(props.content as RichText),
  extensions: [
    StarterKit.configure({ blockquote: false, heading: false, horizontalRule: false, link: false, strike: false }),
    Link.configure({
      openOnClick: false,
      autolink: false,
      linkOnPaste: true,
      defaultProtocol: 'https',
      isAllowedUri: (url) => safePublishedURL(url) !== undefined,
      HTMLAttributes: { rel: 'noopener noreferrer' },
    }),
  ],
  editorProps: { attributes: { 'aria-label': props.label, 'aria-multiline': 'true', class: 'authoring-rich-text__surface', role: 'textbox' } },
  onUpdate: ({ editor: current }) => {
    if (!applyingExternalValue) emit('update:content', editorDocumentToRichText(current.getJSON()))
  },
  onTransaction: () => { editorRevision.value += 1 },
})

const empty = computed(() => { editorRevision.value; return editor.isEmpty })

watch(() => props.label, (label) => editor.setOptions({ editorProps: { attributes: { 'aria-label': label, 'aria-multiline': 'true', class: 'authoring-rich-text__surface', role: 'textbox' } } }))
watch(() => props.content, (content) => {
  if (JSON.stringify(editorDocumentToRichText(editor.getJSON())) === JSON.stringify(content)) return
  applyingExternalValue = true
  editor.commands.setContent(richTextToEditorDocument(content as RichText), { emitUpdate: false })
  applyingExternalValue = false
}, { deep: true })
onBeforeUnmount(() => editor.destroy())

function openLink(event: Event) {
  linkTrigger.value = event.currentTarget instanceof HTMLElement ? event.currentTarget : undefined
  linkURL.value = editor.getAttributes('link').href ?? ''
  linkError.value = undefined
  linkOpen.value = true
  void nextTick(() => document.getElementById('authoring-rich-text-link')?.focus())
}
function closeLink() { linkOpen.value = false; linkError.value = undefined; void nextTick(() => linkTrigger.value?.focus()) }
function applyLink() {
  const href = safePublishedURL(linkURL.value.trim())
  if (!href) { linkError.value = t('authoring.content.richText.invalidLink'); return }
  editor.chain().focus().extendMarkRange('link').setLink({ href }).run()
  closeLink()
}
function removeLink() { editor.chain().focus().extendMarkRange('link').unsetLink().run(); closeLink() }
function active(name: string) { editorRevision.value; return editor.isActive(name) }
</script>

<template>
  <div class="authoring-rich-text">
    <div :class="['authoring-rich-text__frame', { 'is-empty': empty }]">
      <div class="authoring-rich-text__toolbar" role="toolbar" :aria-label="`${label} ${t('authoring.content.richText.toolbar')}`">
        <div class="authoring-rich-text__tool-group" role="group" :aria-label="t('authoring.content.richText.inline')">
          <BtgIconButton icon="bold" :label="t('authoring.content.richText.bold')" :title="t('authoring.content.richText.bold')" :aria-pressed="active('bold')" @click="editor.chain().focus().toggleBold().run()" />
          <BtgIconButton icon="italic" :label="t('authoring.content.richText.italic')" :aria-pressed="active('italic')" @click="editor.chain().focus().toggleItalic().run()" />
        </div>
        <div class="authoring-rich-text__tool-group authoring-rich-text__link-control" role="group" :aria-label="t('authoring.content.richText.linkFormatting')">
          <BtgIconButton icon="link" :label="t('authoring.content.richText.link')" :aria-pressed="active('link')" @click="openLink" />
          <div v-if="linkOpen" class="authoring-rich-text__link" role="group" :aria-label="t('authoring.content.richText.linkEditor')">
            <label for="authoring-rich-text-link">{{ t('authoring.content.richText.linkUrl') }}</label>
            <BtgTextInput id="authoring-rich-text-link" v-model="linkURL" type="url" :placeholder="t('authoring.content.richText.linkPlaceholder')" :aria-invalid="linkError ? 'true' : undefined" :aria-describedby="linkError ? 'authoring-rich-text-link-error' : undefined" @keydown.esc="closeLink" @keydown.enter.prevent="applyLink" />
            <p v-if="linkError" id="authoring-rich-text-link-error" role="alert">{{ linkError }}</p>
            <div><BtgButton type="button" variant="secondary" @click="closeLink">{{ t('authoring.content.richText.cancel') }}</BtgButton><BtgButton v-if="active('link')" type="button" variant="danger-secondary" @click="removeLink">{{ t('authoring.content.richText.removeLink') }}</BtgButton><BtgButton type="button" @click="applyLink">{{ t('authoring.content.richText.applyLink') }}</BtgButton></div>
          </div>
        </div>
        <div class="authoring-rich-text__tool-group" role="group" :aria-label="t('authoring.content.richText.listFormatting')">
          <BtgIconButton icon="list" :label="t('authoring.content.richText.bulletList')" :aria-pressed="active('bulletList')" @click="editor.chain().focus().toggleBulletList().run()" />
          <BtgIconButton icon="listOrdered" :label="t('authoring.content.richText.numberedList')" :aria-pressed="active('orderedList')" @click="editor.chain().focus().toggleOrderedList().run()" />
        </div>
        <div class="authoring-rich-text__tool-group" role="group" :aria-label="t('authoring.content.richText.codeFormatting')">
          <BtgIconButton icon="code" :label="t('authoring.content.richText.inlineCode')" :aria-pressed="active('code')" @click="editor.chain().focus().toggleCode().run()" />
          <BtgIconButton v-if="codeBlocks" icon="code" :label="t('authoring.content.richText.codeBlock')" :title="t('authoring.content.richText.codeBlock')" :aria-pressed="active('codeBlock')" @click="editor.chain().focus().toggleCodeBlock().run()" />
        </div>
      </div>
      <EditorContent :editor="editor" />
    </div>
    <p v-if="empty" class="authoring-rich-text__error" role="alert">{{ t('authoring.content.richText.empty') }}</p>
    <p class="authoring-rich-text__help">{{ t('authoring.content.richText.help') }}</p>
  </div>
</template>
