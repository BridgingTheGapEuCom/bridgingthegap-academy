<template>
  <span ref="root" class="btg-menu-root">
    <DropdownMenuRoot :open="open" @update:open="onOpenChange">
      <DropdownMenuTrigger as-child>
        <slot name="trigger"><BtgIconButton icon="overflow" :label="label" /></slot>
      </DropdownMenuTrigger>
      <DropdownMenuPortal>
        <DropdownMenuContent class="btg-menu" :side-offset="8" @escape-key-down="$emit('escape')">
        <template v-for="item in items" :key="item.value">
          <DropdownMenuSeparator v-if="item.separatorBefore" class="btg-menu__separator" />
          <DropdownMenuItem as-child :disabled="item.disabled" @select="$emit('select', item.value)">
            <button type="button" class="btg-menu__item" :class="{ 'btg-menu__item--danger': item.danger }" :disabled="item.disabled">
              <BtgIcon v-if="item.icon" :name="item.icon" decorative size="small" />{{ item.label }}
            </button>
          </DropdownMenuItem>
        </template>
        </DropdownMenuContent>
      </DropdownMenuPortal>
    </DropdownMenuRoot>
  </span>
</template>

<script setup lang="ts">
import { DropdownMenuContent, DropdownMenuItem, DropdownMenuPortal, DropdownMenuRoot, DropdownMenuSeparator, DropdownMenuTrigger } from 'reka-ui'
import { nextTick, ref } from 'vue'
import BtgIcon from './BtgIcon.vue'
import BtgIconButton from './BtgIconButton.vue'
import type { BtgIconName } from './btg-icon-names'

const open = defineModel<boolean>('open', { default: false })
defineProps<{ label: string; items: { value: string; label: string; icon?: BtgIconName; disabled?: boolean; danger?: boolean; separatorBefore?: boolean }[] }>()
defineEmits<{ select: [value: string]; escape: [] }>()
const root = ref<HTMLElement>()

function onOpenChange(next: boolean) {
  open.value = next
  if (!next) void nextTick(() => root.value?.querySelector<HTMLElement>('[aria-haspopup="menu"]')?.focus())
}
</script>
