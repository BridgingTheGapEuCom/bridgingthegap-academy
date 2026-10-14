<template>
  <fieldset class="translation-field">
    <legend>{{ label }}</legend>
    <BtgSplitWorkspace :primary-label="copy.sourceRegion" :secondary-label="copy.targetRegion">
      <template #primary>
        <div class="translation-field__pane translation-field__pane--source">
          <header>
            <strong>{{ copy.source }}</strong>
            <BtgBadge>{{ copy.readOnly }}</BtgBadge>
          </header>
          <p class="translation-field__source">{{ field.source }}</p>
        </div>
      </template>
      <template #secondary>
        <div class="translation-field__pane translation-field__pane--target">
          <header>
            <strong>{{ copy.translation }}</strong>
            <BtgBadge :variant="field.state === 'TRANSLATED' ? 'success' : 'neutral'">
              {{ field.state === 'TRANSLATED' ? copy.translated : copy.untranslated }}
            </BtgBadge>
          </header>
          <BtgFormField :label="copy.inputLabel" :control-id="inputId" class="translation-field__control">
            <template #default="{ controlId, describedBy }">
              <BtgTextarea v-if="multiline" :id="controlId" v-model="value" :aria-describedby="describedBy" @update:model-value="emitValue" />
              <BtgTextInput v-else :id="controlId" v-model="value" :aria-describedby="describedBy" @update:model-value="emitValue" />
            </template>
          </BtgFormField>
          <BtgButton type="button" variant="secondary" size="small" :disabled="field.state === 'UNTRANSLATED' && value === ''" @click="markUntranslated">
            {{ copy.markUntranslated }}
          </BtgButton>
        </div>
      </template>
    </BtgSplitWorkspace>
  </fieldset>
</template>

<script setup lang="ts">
import { ref, useId, watch } from 'vue'
import type { TextField } from '../translations/translations'
import BtgBadge from './BtgBadge.vue'
import BtgButton from './BtgButton.vue'
import BtgFormField from './BtgFormField.vue'
import BtgSplitWorkspace from './BtgSplitWorkspace.vue'
import BtgTextInput from './BtgTextInput.vue'
import BtgTextarea from './BtgTextarea.vue'

export type TranslationFieldCopy = {
  sourceRegion: string
  targetRegion: string
  source: string
  translation: string
  readOnly: string
  untranslated: string
  translated: string
  markUntranslated: string
  inputLabel: string
}

const props = withDefaults(defineProps<{ label: string; field: TextField; copy: TranslationFieldCopy; multiline?: boolean; resetKey?: number }>(), {
  multiline: false,
  resetKey: 0,
})
const emit = defineEmits<{ change: [value: string | null] }>()
const inputId = `translation-field-${useId()}`
const value = ref(props.field.translated ?? '')

watch([() => props.field, () => props.resetKey], () => { value.value = props.field.translated ?? '' })
function emitValue(next: string) { emit('change', next) }
function markUntranslated() { value.value = ''; emit('change', null) }
</script>
