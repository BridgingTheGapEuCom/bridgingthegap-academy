<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import Link from '@tiptap/extension-link'
import StarterKit from '@tiptap/starter-kit'
import { Editor, EditorContent } from '@tiptap/vue-3'
import type { components } from '../api/generated'
import type { RichText } from '../lesson/content'
import { safePublishedURL } from '../lesson/content'
import { editorDocumentToRichText, richTextToEditorDocument } from '../authoring/richText'

type APIRichText = components['schemas']['RichText']
const props = defineProps<{ content: APIRichText; label: string }>()
const emit = defineEmits<{ 'update:content': [content: APIRichText] }>()
const linkOpen = ref(false)
const linkURL = ref('')
const linkError = ref<string>()
const editorRevision = ref(0)
let applyingExternalValue = false

const editor = new Editor({
  content: richTextToEditorDocument(props.content as RichText),
  extensions: [
    StarterKit.configure({ blockquote: false, codeBlock: false, heading: false, horizontalRule: false, link: false, strike: false }),
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

function openLink() {
  linkURL.value = editor.getAttributes('link').href ?? ''
  linkError.value = undefined
  linkOpen.value = true
  void nextTick(() => document.getElementById('authoring-rich-text-link')?.focus())
}
function applyLink() {
  const href = safePublishedURL(linkURL.value.trim())
  if (!href) { linkError.value = 'Use a safe HTTPS or internal URL.'; return }
  editor.chain().focus().extendMarkRange('link').setLink({ href }).run()
  linkOpen.value = false
}
function removeLink() { editor.chain().focus().extendMarkRange('link').unsetLink().run(); linkOpen.value = false }
function active(name: string) { editorRevision.value; return editor.isActive(name) }
</script>

<template>
  <div class="authoring-rich-text">
    <div class="authoring-rich-text__toolbar" role="toolbar" :aria-label="`${label} formatting`">
      <button type="button" :aria-pressed="active('bold')" title="Bold (Ctrl+B)" @click="editor.chain().focus().toggleBold().run()"><strong aria-hidden="true">B</strong><span class="sr-only">Bold</span></button>
      <button type="button" :aria-pressed="active('italic')" title="Italic (Ctrl+I)" @click="editor.chain().focus().toggleItalic().run()"><em aria-hidden="true">I</em><span class="sr-only">Italic</span></button>
      <button type="button" :aria-pressed="active('link')" @click="openLink">Link</button>
      <button type="button" :aria-pressed="active('bulletList')" @click="editor.chain().focus().toggleBulletList().run()">Bulleted list</button>
      <button type="button" :aria-pressed="active('orderedList')" @click="editor.chain().focus().toggleOrderedList().run()">Numbered list</button>
      <button type="button" :aria-pressed="active('code')" @click="editor.chain().focus().toggleCode().run()">Inline code</button>
    </div>
    <div v-if="linkOpen" class="authoring-rich-text__link">
      <label for="authoring-rich-text-link">Link URL</label>
      <input id="authoring-rich-text-link" v-model="linkURL" type="url" placeholder="https://example.com or /course/page" :aria-invalid="linkError ? 'true' : undefined" :aria-describedby="linkError ? 'authoring-rich-text-link-error' : undefined" @keydown.esc="linkOpen = false" @keydown.enter.prevent="applyLink" />
      <p v-if="linkError" id="authoring-rich-text-link-error" role="alert">{{ linkError }}</p>
      <div><button type="button" @click="linkOpen = false">Cancel</button><button v-if="active('link')" type="button" @click="removeLink">Remove link</button><button type="button" @click="applyLink">Apply link</button></div>
    </div>
    <EditorContent :editor="editor" />
    <p v-if="empty" class="authoring-rich-text__error" role="alert">Enter text for this block.</p>
    <p class="authoring-rich-text__help">Use paragraphs, lists, emphasis, links, and inline code. Lesson headings belong in Heading blocks.</p>
  </div>
</template>
