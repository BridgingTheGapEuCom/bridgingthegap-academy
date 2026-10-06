<script setup lang="ts">
import type { components } from '../api/generated'
import BtgFormField from './BtgFormField.vue'
import BtgTextarea from './BtgTextarea.vue'
type Inline = components['schemas']['RichTextInline']
const props = defineProps<{ items: Inline[]; label: string; copy: { textLabel: (index: number) => string; preservedFormatting: string; textRequired: string; hardBreakPreserved: string } }>()
const emit = defineEmits<{ 'update:items': [items: Inline[]] }>()
function edit(index: number, value: string) {
  emit('update:items', props.items.map((item, i) => i === index ? { ...item, text: value } : item))
}
</script>

<template>
  <div class="authoring-rich-text">
    <template v-for="(inline, index) in items" :key="index">
      <BtgFormField v-if="inline.type === 'text'" :label="copy.textLabel(index + 1)" :description="inline.marks?.length ? copy.preservedFormatting : undefined" :error="inline.text === '' ? copy.textRequired : undefined" v-slot="{ controlId, describedBy, invalid }">
        <BtgTextarea :id="controlId" :model-value="inline.text" :aria-describedby="describedBy" :invalid="invalid" maxlength="100000" required @update:model-value="edit(index, $event)" />
      </BtgFormField>
      <p v-else-if="inline.type === 'hard_break'" class="authoring-section__intro">{{ copy.hardBreakPreserved }}</p>
    </template>
  </div>
</template>
