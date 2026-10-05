<template>
  <nav class="btg-previous-next" :aria-label="navigationLabel">
    <BtgButton variant="secondary" :disabled="previousDisabled" leading-icon="previous" @click="$emit('previous')">{{ previousLabel }}</BtgButton>
    <BtgButton variant="secondary" :disabled="nextDisabled" trailing-icon="next" @click="$emit('next')">{{ nextLabel }}</BtgButton>
  </nav>
</template>

<script setup lang="ts">
import BtgButton from './BtgButton.vue'

withDefaults(defineProps<{ navigationLabel: string; previousDisabled?: boolean; nextDisabled?: boolean; previousLabel: string; nextLabel: string }>(), {
  previousDisabled: false,
  nextDisabled: false,
})
defineEmits<{ previous: []; next: [] }>()
</script>
