<template>
  <nav v-if="total > limit" class="community-pagination" :aria-label="copy.label">
    <p class="community-pagination__summary" aria-live="polite">{{ copy.range(start, end, total) }}</p>
    <div class="community-pagination__actions">
      <BtgButton variant="secondary" leading-icon="previous" :disabled="pending || offset === 0" @click="emit('previous')">{{ copy.previous }}</BtgButton>
      <BtgButton variant="secondary" trailing-icon="next" :disabled="pending || end >= total" @click="emit('next')">{{ copy.next }}</BtgButton>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import BtgButton from './BtgButton.vue'

export interface CommunityPaginationCopy {
  label: string
  range: (start: number, end: number, total: number) => string
  previous: string
  next: string
}

const props = withDefaults(defineProps<{
  limit: number
  offset: number
  total: number
  pending?: boolean
  copy: CommunityPaginationCopy
}>(), { pending: false })
const emit = defineEmits<{ previous: []; next: [] }>()
const start = computed(() => props.total === 0 ? 0 : props.offset + 1)
const end = computed(() => Math.min(props.offset + props.limit, props.total))
</script>
