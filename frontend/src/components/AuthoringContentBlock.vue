<script setup lang="ts">
import type { CanonicalBlock } from '../authoring/contentEditor'
import BtgFormField from './BtgFormField.vue'
import AuthoringRichTextEditor from './AuthoringRichTextEditor.vue'
import AuthoringInlineTextEditor from './AuthoringInlineTextEditor.vue'
const props = defineProps<{ block: CanonicalBlock; position: number }>()
const emit = defineEmits<{ update: [block: CanonicalBlock] }>()
function field(name: 'code' | 'language' | 'title' | 'text' | 'attribution' | 'sourceUrl', event: Event) {
  const value = (event.target as HTMLInputElement).value
  if (props.block.type === 'CODE' || props.block.type === 'QUOTE' || props.block.type === 'CALLOUT') {
    const payload = { ...props.block.payload, [name]: value }
    if (!value && name !== 'code' && name !== 'text') delete payload[name as keyof typeof payload]
    emit('update', { ...props.block, payload } as CanonicalBlock)
  }
}
</script>

<template>
  <AuthoringRichTextEditor v-if="block.type === 'TEXT'" :content="block.payload.content" :label="`Block ${position} text`" @update:content="emit('update', { ...block, payload: { ...block.payload, content: $event } })" />
  <template v-else-if="block.type === 'HEADING'">
    <BtgFormField label="Heading level" v-slot="{ controlId }">
      <select :id="controlId" :value="block.payload.level" @change="emit('update', { ...block, payload: { ...block.payload, level: Number(($event.target as HTMLSelectElement).value) } })"><option :value="2">Heading 2</option><option :value="3">Heading 3</option><option :value="4">Heading 4</option></select>
    </BtgFormField>
    <AuthoringInlineTextEditor :items="block.payload.content" :label="`Block ${position} heading`" @update:items="emit('update', { ...block, payload: { ...block.payload, content: $event } })" />
  </template>
  <template v-else-if="block.type === 'CODE'">
    <BtgFormField label="Code" :error="!block.payload.code ? 'Enter code to display.' : undefined" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" :value="block.payload.code" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" spellcheck="false" maxlength="100000" required @input="field('code', $event)" /></BtgFormField>
    <BtgFormField label="Code language" v-slot="{ controlId }"><input :id="controlId" :value="block.payload.language" maxlength="64" @input="field('language', $event)" /></BtgFormField>
    <BtgFormField label="Code title" v-slot="{ controlId }"><input :id="controlId" :value="block.payload.title" maxlength="240" @input="field('title', $event)" /></BtgFormField>
  </template>
  <template v-else-if="block.type === 'QUOTE'">
    <BtgFormField label="Quote text" :error="!block.payload.text.trim() ? 'Enter quote text.' : undefined" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" :value="block.payload.text" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" maxlength="50000" required @input="field('text', $event)" /></BtgFormField>
    <BtgFormField label="Quote attribution" v-slot="{ controlId }"><input :id="controlId" :value="block.payload.attribution" maxlength="240" @input="field('attribution', $event)" /></BtgFormField>
    <BtgFormField label="Quote source URL" v-slot="{ controlId }"><input :id="controlId" :value="block.payload.sourceUrl" maxlength="2048" @input="field('sourceUrl', $event)" /></BtgFormField>
  </template>
  <template v-else-if="block.type === 'CALLOUT'">
    <BtgFormField label="Callout kind" v-slot="{ controlId }"><select :id="controlId" :value="block.payload.kind" @change="emit('update', { ...block, payload: { ...block.payload, kind: ($event.target as HTMLSelectElement).value as 'INFO' | 'NOTE' | 'WARNING' | 'TIP' } })"><option value="INFO">Information</option><option value="NOTE">Note</option><option value="WARNING">Warning</option><option value="TIP">Tip</option></select></BtgFormField>
    <BtgFormField label="Callout title" v-slot="{ controlId }"><input :id="controlId" :value="block.payload.title" maxlength="240" @input="field('title', $event)" /></BtgFormField>
    <AuthoringRichTextEditor :content="block.payload.content" :label="`Block ${position} callout`" @update:content="emit('update', { ...block, payload: { ...block.payload, content: $event } })" />
  </template>
  <p v-else-if="block.type === 'DIVIDER'" class="authoring-section__intro">A semantic divider. No configuration is needed.</p>
  <template v-else>
    <p>This {{ block.type.toLowerCase().replaceAll('_', ' ') }} block is preserved read-only. You can move or remove it.</p>
    <details><summary>Preserved canonical payload</summary><pre class="authoring-content__payload">{{ JSON.stringify(block.payload, null, 2) }}</pre></details>
  </template>
</template>
