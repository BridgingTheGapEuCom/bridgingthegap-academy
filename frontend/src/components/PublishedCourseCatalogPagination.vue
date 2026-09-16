<template>
  <nav v-if="total > 0" class="course-catalog-pagination" aria-label="Course catalog pagination">
    <p class="course-catalog-pagination__summary" aria-live="polite">Showing {{ start }}–{{ end }} of {{ total }} courses</p>
    <div class="course-catalog-pagination__actions">
      <BtgButton variant="secondary" :disabled="offset === 0 || pending" @click="emit('previous')">Previous</BtgButton>
      <BtgButton variant="secondary" :disabled="end >= total || pending" @click="emit('next')">Next</BtgButton>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import BtgButton from './BtgButton.vue'

const props = defineProps<{ limit: number; offset: number; total: number; pending?: boolean }>()
const emit = defineEmits<{ previous: []; next: [] }>()

const start = computed(() => props.total === 0 ? 0 : props.offset + 1)
const end = computed(() => Math.min(props.offset + props.limit, props.total))
</script>
