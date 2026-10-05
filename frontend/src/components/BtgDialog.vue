<template>
  <DialogRoot v-model:open="open">
    <DialogTrigger v-if="showTrigger" as-child>
      <slot name="trigger"><BtgButton v-if="triggerLabel">{{ triggerLabel }}</BtgButton></slot>
    </DialogTrigger>
    <DialogPortal>
      <DialogOverlay class="btg-dialog__overlay" />
      <DialogContent class="btg-dialog__content">
        <header class="btg-dialog__header">
          <DialogTitle class="btg-dialog__title"><slot name="title">{{ title }}</slot></DialogTitle>
          <DialogDescription v-if="description || $slots.description" class="btg-dialog__description"><slot name="description">{{ description }}</slot></DialogDescription>
        </header>
        <div class="btg-dialog__body"><slot /></div>
        <footer v-if="$slots.footer || dismissLabel" class="btg-dialog__footer">
          <slot name="footer"><DialogClose as-child><BtgButton variant="secondary">{{ dismissLabel }}</BtgButton></DialogClose></slot>
        </footer>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle, DialogTrigger } from 'reka-ui'
import BtgButton from './BtgButton.vue'

const open = defineModel<boolean>('open', { default: false })
withDefaults(defineProps<{ title: string; description?: string; triggerLabel?: string; dismissLabel?: string; showTrigger?: boolean }>(), {
  description: undefined,
  triggerLabel: undefined,
  dismissLabel: undefined,
  showTrigger: true,
})
</script>
