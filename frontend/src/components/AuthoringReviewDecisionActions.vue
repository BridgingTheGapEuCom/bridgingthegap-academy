<template>
  <section class="authoring-review__decisions" aria-labelledby="authoring-review-decisions-title">
    <h3 id="authoring-review-decisions-title">Decide this Review</h3>
    <p>Each decision finishes this Review cycle. It does not publish the Draft or change the frozen snapshot.</p>
    <p v-if="busy" class="authoring-review__status" role="status">{{ pendingLabel }} this Review…</p>
    <div v-if="policyConflict" class="authoring-review__conflict" role="alert">
      <p>This Review must be decided by someone other than the person who submitted it.</p>
    </div>
    <div v-else-if="conflict" class="authoring-review__conflict" role="alert">
      <p>This Review changed before your decision was saved. Reload the latest Review state.</p>
      <BtgButton variant="secondary" :disabled="busy || reloading" @click="$emit('reload')">{{ reloading ? 'Reloading…' : 'Reload review' }}</BtgButton>
    </div>
    <p v-else-if="error" class="authoring-review__error" role="alert">{{ error }}</p>
    <div class="authoring-review__decision-actions">
      <BtgButton :disabled="busy || reloading || conflict || policyConflict" @click="$emit('approve')">Approve</BtgButton>
      <BtgButton variant="destructive" :disabled="busy || reloading || conflict || policyConflict" @click="$emit('request-changes')">Request changes</BtgButton>
    </div>
  </section>
</template>

<script setup lang="ts">
import BtgButton from './BtgButton.vue'

defineProps<{
  busy: boolean
  pendingLabel: 'Approving' | 'Requesting changes'
  reloading: boolean
  policyConflict: boolean
  conflict: boolean
  error?: string
}>()

defineEmits<{
  approve: []
  'request-changes': []
  reload: []
}>()
</script>
