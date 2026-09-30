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
const props = defineProps<{ content: APIRichText; label: string; codeBlocks?: boolean }>()
const emit = defineEmits<{ 'update:content': [content: APIRichText] }>()
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
  if (!href) { linkError.value = 'Use a safe HTTPS or internal URL.'; return }
  editor.chain().focus().extendMarkRange('link').setLink({ href }).run()
  closeLink()
}
function removeLink() { editor.chain().focus().extendMarkRange('link').unsetLink().run(); closeLink() }
function active(name: string) { editorRevision.value; return editor.isActive(name) }
</script>

<template>
  <div class="authoring-rich-text">
    <div :class="['authoring-rich-text__frame', { 'is-empty': empty }]">
      <div class="authoring-rich-text__toolbar" role="toolbar" :aria-label="`${label} formatting`">
        <div class="authoring-rich-text__tool-group" role="group" aria-label="Inline formatting">
          <button type="button" :aria-pressed="active('bold')" aria-label="Bold" title="Bold (Ctrl+B)" @click="editor.chain().focus().toggleBold().run()"><svg aria-hidden="true" viewBox="0 0 24 24"><path d="M7 5h6a4 4 0 0 1 0 8H7zm0 8h7a4 4 0 0 1 0 8H7z" /></svg></button>
          <button type="button" :aria-pressed="active('italic')" aria-label="Italic" title="Italic (Ctrl+I)" @click="editor.chain().focus().toggleItalic().run()"><svg aria-hidden="true" viewBox="0 0 24 24"><path d="M10 5h8M6 19h8M14 5 10 19" /></svg></button>
        </div>
        <div class="authoring-rich-text__tool-group authoring-rich-text__link-control" role="group" aria-label="Link formatting">
          <button type="button" :aria-pressed="active('link')" aria-label="Link" title="Link" @click="openLink"><svg aria-hidden="true" viewBox="0 0 24 24"><path d="m10 13 4-4M8 16l-1 1a3.5 3.5 0 0 1-5-5l4-4a3.5 3.5 0 0 1 5 0M16 8l1-1a3.5 3.5 0 1 1 5 5l-4 4a3.5 3.5 0 0 1-5 0" /></svg></button>
          <div v-if="linkOpen" class="authoring-rich-text__link" role="group" aria-label="Link editor">
            <label for="authoring-rich-text-link">Link URL</label>
            <input id="authoring-rich-text-link" v-model="linkURL" type="url" placeholder="https://example.com or /course/page" :aria-invalid="linkError ? 'true' : undefined" :aria-describedby="linkError ? 'authoring-rich-text-link-error' : undefined" @keydown.esc="closeLink" @keydown.enter.prevent="applyLink" />
            <p v-if="linkError" id="authoring-rich-text-link-error" role="alert">{{ linkError }}</p>
            <div><button type="button" @click="closeLink">Cancel</button><button v-if="active('link')" type="button" @click="removeLink">Remove link</button><button type="button" @click="applyLink">Apply link</button></div>
          </div>
        </div>
        <div class="authoring-rich-text__tool-group" role="group" aria-label="List formatting">
          <button type="button" :aria-pressed="active('bulletList')" aria-label="Bulleted list" title="Bulleted list" @click="editor.chain().focus().toggleBulletList().run()"><svg aria-hidden="true" viewBox="0 0 24 24"><path d="M9 6h11M9 12h11M9 18h11M4 6h.01M4 12h.01M4 18h.01" /></svg></button>
          <button type="button" :aria-pressed="active('orderedList')" aria-label="Numbered list" title="Numbered list" @click="editor.chain().focus().toggleOrderedList().run()"><svg aria-hidden="true" viewBox="0 0 24 24"><path d="M10 6h10M10 12h10M10 18h10M4 5h1v3M3.5 14h2l-2 3h2M3.5 20h2l-2 3h2" /></svg></button>
        </div>
        <div class="authoring-rich-text__tool-group" role="group" aria-label="Code formatting">
          <button type="button" :aria-pressed="active('code')" aria-label="Inline code" title="Inline code" @click="editor.chain().focus().toggleCode().run()"><svg aria-hidden="true" viewBox="0 0 24 24"><path d="m9 6-5 6 5 6M15 6l5 6-5 6" /></svg></button>
          <button v-if="codeBlocks" type="button" :aria-pressed="active('codeBlock')" aria-label="Code block" title="Code block" @click="editor.chain().focus().toggleCodeBlock().run()"><svg aria-hidden="true" viewBox="0 0 24 24"><path d="M4 5h16v14H4zM8 9l-2 3 2 3M11 15h5" /></svg></button>
        </div>
      </div>
      <EditorContent :editor="editor" />
    </div>
    <p v-if="empty" class="authoring-rich-text__error" role="alert">Enter text for this block.</p>
    <p class="authoring-rich-text__help">Use paragraphs, lists, emphasis, links, inline code<template v-if="codeBlocks">, and small multiline code blocks</template>. Lesson headings belong in Heading blocks.</p>
  </div>
</template>
