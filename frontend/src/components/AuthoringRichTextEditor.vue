<script setup lang="ts">
import type { components } from '../api/generated'
import AuthoringInlineTextEditor from './AuthoringInlineTextEditor.vue'
type RichText = components['schemas']['RichText']
type Inline = components['schemas']['RichTextInline']
const props = defineProps<{ content: RichText; label: string }>()
const emit = defineEmits<{ 'update:content': [content: RichText] }>()
function paragraph(index: number, content: Inline[]) {
  emit('update:content', { ...props.content, nodes: props.content.nodes.map((node, i) => i === index ? { ...node, content } : node) })
}
function listItem(index: number, itemIndex: number, content: Inline[]) {
  emit('update:content', { ...props.content, nodes: props.content.nodes.map((node, i) => i === index ? { ...node, items: node.items?.map((item, j) => j === itemIndex ? content : item) } : node) })
}
</script>

<template>
  <div class="authoring-rich-text">
    <p class="authoring-section__intro">Edit the existing text runs. Paragraphs, lists, formatting, links, and hard breaks are preserved.</p>
    <template v-for="(node, index) in content.nodes" :key="index">
      <AuthoringInlineTextEditor v-if="node.type === 'paragraph'" :items="node.content ?? []" :label="`${label} · Paragraph ${index + 1}`" @update:items="paragraph(index, $event)" />
      <fieldset v-else-if="node.type === 'bullet_list' || node.type === 'ordered_list'" class="authoring-rich-text__list">
        <legend>{{ node.type === 'bullet_list' ? 'Bullet list' : 'Ordered list' }} {{ index + 1 }}</legend>
        <AuthoringInlineTextEditor v-for="(item, itemIndex) in node.items" :key="itemIndex" :items="item" :label="`${label} · List ${index + 1} · Item ${itemIndex + 1}`" @update:items="listItem(index, itemIndex, $event)" />
      </fieldset>
    </template>
  </div>
</template>
