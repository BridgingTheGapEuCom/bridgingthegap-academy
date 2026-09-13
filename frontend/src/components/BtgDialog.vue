<template>
  <DialogRoot>
    <DialogTrigger as-child><BtgButton><slot name="trigger" /></BtgButton></DialogTrigger>
    <DialogPortal>
      <DialogOverlay class="dialog-overlay" />
      <DialogContent class="dialog-content">
        <DialogTitle><slot name="title" /></DialogTitle>
        <DialogDescription><slot name="description" /></DialogDescription>
        <slot />
        <DialogClose as-child><BtgButton>Close</BtgButton></DialogClose>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
import { DialogRoot, DialogTrigger, DialogPortal, DialogOverlay, DialogContent, DialogTitle, DialogDescription, DialogClose } from 'reka-ui'
import BtgButton from './BtgButton.vue'
</script>
