<script setup lang="ts">
import type { components } from '../api/generated'
import BtgFormField from './BtgFormField.vue'
type Inline = components['schemas']['RichTextInline']
const props = defineProps<{ items: Inline[]; label: string }>()
const emit = defineEmits<{ 'update:items': [items: Inline[]] }>()
function edit(index: number, event: Event) {
  emit('update:items', props.items.map((item, i) => i === index ? { ...item, text: (event.target as HTMLTextAreaElement).value } : item))
}
</script>

<template>
  <div class="authoring-rich-text">
    <template v-for="(inline, index) in items" :key="index">
      <BtgFormField v-if="inline.type === 'text'" :label="`${label} · Text ${index + 1}`" :description="inline.marks?.length ? 'Existing formatting and links are preserved.' : undefined" :error="inline.text === '' ? 'Enter text for this run.' : undefined" v-slot="{ controlId, describedBy, invalid }">
        <textarea :id="controlId" :value="inline.text" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" maxlength="100000" required @input="edit(index, $event)" />
      </BtgFormField>
      <p v-else-if="inline.type === 'hard_break'" class="authoring-section__intro">Hard break (preserved)</p>
    </template>
  </div>
</template>
