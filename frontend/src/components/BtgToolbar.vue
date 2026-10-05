<template>
  <div class="btg-toolbar">
    <div v-if="$slots.leading" class="btg-toolbar__leading"><slot name="leading" /></div>
    <div v-if="$slots.default" class="btg-toolbar__content"><slot /></div>
    <div v-if="$slots.actions" class="btg-toolbar__actions"><slot name="actions" /></div>
  </div>
</template>
