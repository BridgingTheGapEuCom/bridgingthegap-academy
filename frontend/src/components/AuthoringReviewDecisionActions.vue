<template>
  <BtgPanel as="section" class="authoring-review__decisions" aria-labelledby="authoring-review-decisions-title">
    <h3 id="authoring-review-decisions-title">{{ t('authoring.review.decisions.title') }}</h3>
    <p>{{ t('authoring.review.decisions.description') }}</p>
    <p v-if="busy" class="authoring-review__status" role="status">{{ t('authoring.review.decisions.pending', { action: pendingLabel }) }}</p>
    <div v-if="policyConflict" class="authoring-review__conflict" role="alert">
      <p>{{ t('authoring.review.decisions.policyConflict') }}</p>
    </div>
    <div v-else-if="conflict" class="authoring-review__conflict" role="alert">
      <p>{{ t('authoring.review.decisions.conflict') }}</p>
      <BtgButton variant="secondary" :disabled="busy || reloading" @click="$emit('reload')">{{ reloading ? t('authoring.review.decisions.reloading') : t('authoring.review.decisions.reload') }}</BtgButton>
    </div>
    <p v-else-if="error" class="authoring-review__error" role="alert">{{ error }}</p>
    <div class="authoring-review__decision-actions">
      <BtgButton :disabled="busy || reloading || conflict || policyConflict" @click="$emit('approve')">{{ t('authoring.review.decisions.approve') }}</BtgButton>
      <BtgButton variant="danger-secondary" :disabled="busy || reloading || conflict || policyConflict" @click="$emit('request-changes')">{{ t('authoring.review.decisions.requestChanges') }}</BtgButton>
    </div>
  </BtgPanel>
</template>

<script setup lang="ts">
import BtgButton from './BtgButton.vue'
import BtgPanel from './BtgPanel.vue'
import { useI18n } from 'vue-i18n'

defineProps<{
  busy: boolean
  pendingLabel: string
  reloading: boolean
  policyConflict: boolean
  conflict: boolean
  error?: string
}>()

const { t } = useI18n()

defineEmits<{
  approve: []
  'request-changes': []
  reload: []
}>()
</script>
