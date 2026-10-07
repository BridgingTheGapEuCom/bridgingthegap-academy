<template>
  <BtgPanel as="section" class="authoring-review__submission" aria-labelledby="authoring-review-submission-title">
    <h3 id="authoring-review-submission-title">{{ t('authoring.review.submission.title') }}</h3>
    <p>{{ t('authoring.review.submission.description') }}</p>
    <p v-if="busy" class="authoring-review__status" role="status">{{ t('authoring.review.submission.submitting') }}</p>
    <div v-if="conflict" class="authoring-review__conflict" role="status">
      <p>{{ t('authoring.review.submission.conflict') }}</p>
      <BtgButton variant="secondary" :disabled="busy || reloading" @click="$emit('reload')">{{ reloading ? t('authoring.review.reloading') : t('authoring.review.submission.reload') }}</BtgButton>
    </div>
    <BtgButton :disabled="busy || reloading || conflict" @click="$emit('submit')">{{ busy ? t('authoring.review.submission.busy') : t('authoring.review.submission.action') }}</BtgButton>
  </BtgPanel>
</template>

<script setup lang="ts">
import BtgButton from './BtgButton.vue'
import BtgPanel from './BtgPanel.vue'
import { useI18n } from 'vue-i18n'

defineProps<{
  busy: boolean
  reloading: boolean
  conflict: boolean
}>()

defineEmits<{
  submit: []
  reload: []
}>()
const { t } = useI18n()
</script>
