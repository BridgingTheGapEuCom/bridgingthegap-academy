<template>
  <section class="authoring-review__submission" aria-labelledby="authoring-review-submission-title">
    <h3 id="authoring-review-submission-title">Submit for review</h3>
    <p>Submitting creates a Review of the current Draft revision. Later Draft edits do not change that frozen snapshot.</p>
    <p v-if="busy" class="authoring-review__status" role="status">Submitting the current Draft revision for review…</p>
    <div v-if="conflict" class="authoring-review__conflict" role="status">
      <p>This Review or Draft changed before submission could finish. Reload the latest state before trying again.</p>
      <BtgButton variant="secondary" :disabled="busy || reloading" @click="$emit('reload')">{{ reloading ? 'Reloading…' : 'Reload review state' }}</BtgButton>
    </div>
    <BtgButton :disabled="busy || reloading || conflict" @click="$emit('submit')">{{ busy ? 'Submitting…' : 'Submit for review' }}</BtgButton>
  </section>
</template>

<script setup lang="ts">
import BtgButton from './BtgButton.vue'

defineProps<{
  busy: boolean
  reloading: boolean
  conflict: boolean
}>()

defineEmits<{
  submit: []
  reload: []
}>()
</script>
