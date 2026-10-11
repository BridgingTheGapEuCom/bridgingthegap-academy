<template>
  <nav v-if="total > 0" class="course-catalog-pagination" :aria-label="copy.label">
    <p class="course-catalog-pagination__summary" aria-live="polite">{{ copy.range(start, end, total) }}</p>
    <div class="course-catalog-pagination__actions">
      <BtgButton variant="secondary" leading-icon="previous" :disabled="offset === 0 || pending" @click="emit('previous')">{{ copy.previous }}</BtgButton>
      <BtgButton variant="secondary" trailing-icon="next" :disabled="end >= total || pending" @click="emit('next')">{{ copy.next }}</BtgButton>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import BtgButton from './BtgButton.vue'

export interface PublishedCourseCatalogPaginationCopy {
  label: string
  range: (start: number, end: number, total: number) => string
  previous: string
  next: string
}

const props = defineProps<{ limit: number; offset: number; total: number; pending?: boolean; copy: PublishedCourseCatalogPaginationCopy }>()
const emit = defineEmits<{ previous: []; next: [] }>()
const start = computed(() => props.total === 0 ? 0 : props.offset + 1)
const end = computed(() => Math.min(props.offset + props.limit, props.total))
</script>
