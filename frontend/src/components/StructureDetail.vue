<template>
  <article v-if="item" class="authoring-structure-detail-panel">
    <template v-if="item.kind === 'module'">
      <p>Module · {{ item.module.lessons.length }} lessons</p>
      <h3>{{ item.module.title }}</h3>
      <p v-if="item.module.description">{{ item.module.description }}</p>
      <div class="authoring-structure-detail-panel__actions">
        <BtgButton variant="secondary" :disabled="busy" @click="$emit('edit-module', item.module, $event)">Edit module</BtgButton>
        <BtgButton variant="quiet" :disabled="busy || item.module.position === 0" @click="$emit('move-module', -1, item.module)">Move up</BtgButton>
        <BtgButton variant="quiet" :disabled="busy || item.module.position === moduleCount - 1" @click="$emit('move-module', 1, item.module)">Move down</BtgButton>
      </div>
      <div class="authoring-structure-detail-panel__destructive">
        <BtgButton variant="destructive" :disabled="busy || item.module.lessons.length > 0" @click="$emit('delete-module', item.module)">Delete module</BtgButton>
      </div>
    </template>

    <template v-else>
      <p>Lesson · {{ item.module.title }}</p>
      <h3>{{ item.lesson.title }}</h3>
      <section aria-labelledby="structure-lesson-details-heading">
        <h4 id="structure-lesson-details-heading">Lesson details</h4>
        <p>{{ item.lesson.description }}</p>
        <h4>Learning objectives</h4>
        <ul><li v-for="objective in item.lesson.objectives" :key="objective">{{ objective }}</li></ul>
      </section>

      <section class="authoring-structure-detail-panel__content" aria-labelledby="structure-lesson-content-heading">
        <h4 id="structure-lesson-content-heading">Lesson content</h4>
        <p>Build the material learners will see in this lesson.</p>
        <p v-if="content.kind === 'loading'" role="status">Loading lesson content…</p>
        <template v-else-if="content.kind === 'unavailable'">
          <p role="alert">We couldn’t load this lesson’s content summary right now.</p>
          <BtgButton variant="secondary" @click="$emit('retry-content', item.lesson)">Try again</BtgButton>
          <BtgButton @click="$emit('edit-content', item.module, item.lesson)">Edit content</BtgButton>
        </template>
        <template v-else-if="content.blocks.length === 0">
          <p><strong>No lesson content yet.</strong></p>
          <p>Add text, media, code, callouts, and other learning material.</p>
          <BtgButton @click="$emit('edit-content', item.module, item.lesson)">+ Add content</BtgButton>
        </template>
        <template v-else>
          <p><strong>{{ content.blocks.length }} {{ content.blocks.length === 1 ? 'block' : 'blocks' }}</strong></p>
          <p v-if="blockSummary">{{ blockSummary }}</p>
          <BtgButton @click="$emit('edit-content', item.module, item.lesson)">Edit content</BtgButton>
        </template>
      </section>

      <div class="authoring-structure-detail-panel__actions">
        <BtgButton variant="secondary" :disabled="busy" @click="$emit('edit-details', item.module, item.lesson)">Edit details</BtgButton>
        <BtgButton variant="quiet" :disabled="busy" @click="$emit('move-lesson', item.lesson, $event)">Move lesson</BtgButton>
      </div>
      <div class="authoring-structure-detail-panel__destructive">
        <BtgButton variant="destructive" :disabled="busy" @click="$emit('delete-lesson', item.lesson)">Delete lesson</BtgButton>
      </div>
    </template>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AuthoringLessonSummary, AuthoringModuleSummary } from '../authoring/authoring'
import BtgButton from './BtgButton.vue'

type Item = { kind: 'module'; module: AuthoringModuleSummary } | { kind: 'lesson'; module: AuthoringModuleSummary; lesson: AuthoringLessonSummary }
type Content = { kind: 'loading' } | { kind: 'unavailable' } | { kind: 'ready'; blocks: string[] }

const props = defineProps<{ item?: Item; busy: boolean; moduleCount: number; content: Content }>()
defineEmits(['edit-module', 'edit-details', 'edit-content', 'retry-content', 'move-module', 'move-lesson', 'delete-module', 'delete-lesson'])

const blockSummary = computed(() => {
  if (props.content.kind !== 'ready') return ''
  const labels = [...new Set(props.content.blocks)].slice(0, 4).map((type) => type.toLowerCase().split('_').map((part) => `${part[0]?.toUpperCase()}${part.slice(1)}`).join(' '))
  const hidden = new Set(props.content.blocks).size - labels.length
  return hidden > 0 ? `${labels.join(' · ')} · +${hidden} more` : labels.join(' · ')
})
</script>
