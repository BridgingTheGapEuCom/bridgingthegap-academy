<template>
  <section class="authoring-review__decisions authoring-review__publication" aria-labelledby="authoring-review-publication-title">
    <h3 id="authoring-review-publication-title">Publish this Review</h3>
    <p>Publishing creates an immutable CourseVersion from this exact frozen Review snapshot.</p>
    <p v-if="busy" class="authoring-review__status" role="status">Publishing this Review…</p>
    <p v-else-if="error" class="authoring-review__error" role="alert">We couldn’t publish this Review right now. Please try again.</p>
    <BtgButton :disabled="busy" @click="$emit('publish')">{{ busy ? 'Publishing…' : 'Publish course version' }}</BtgButton>
  </section>
</template>

<script setup lang="ts">
import BtgButton from './BtgButton.vue'

defineProps<{
  busy: boolean
  error: boolean
}>()

defineEmits<{
  publish: []
}>()
</script>
