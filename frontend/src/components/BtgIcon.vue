<template>
  <component
    :is="icons[name]"
    class="btg-icon"
    :class="`btg-icon--${size}`"
    :aria-hidden="isDecorative ? 'true' : undefined"
    :aria-label="isDecorative ? undefined : label"
    :role="isDecorative ? undefined : 'img'"
    focusable="false"
  />
</template>

<script setup lang="ts">
import {
  ArrowLeft,
  ArrowRight,
  ArrowRightLeft,
  ChevronDown,
  ChevronRight,
  Ellipsis,
  FileText,
  Folder,
  GripVertical,
  Image,
  Info,
  Layers,
  Minus,
  Pencil,
  Plus,
  Search,
  Settings,
  Target,
  Trash2,
  Type,
  Bold, Italic, Link as LinkIcon, List, ListOrdered, Code, Unlink, Video, AudioLines,
} from '@lucide/vue'
import { computed } from 'vue'
import type { BtgIconName } from './btg-icon-names'

const icons: Record<BtgIconName, typeof Plus> = {
  plus: Plus,
  add: Plus,
  search: Search,
  chevronDown: ChevronDown,
  chevronRight: ChevronRight,
  module: Folder,
  folder: Folder,
  overflow: Ellipsis,
  more: Ellipsis,
  drag: GripVertical,
  lesson: FileText,
  document: FileText,
  previous: ArrowLeft,
  arrowLeft: ArrowLeft,
  next: ArrowRight,
  arrowRight: ArrowRight,
  edit: Pencil,
  objective: Target,
  target: Target,
  content: Layers,
  layers: Layers,
  image: Image,
  video: Video,
  audio: AudioLines,
  divider: Minus,
  text: Type,
  callout: Info,
  info: Info,
  move: ArrowRightLeft,
  settings: Settings,
  delete: Trash2,
  trash: Trash2,
  bold: Bold,
  italic: Italic,
  link: LinkIcon,
  list: List,
  listOrdered: ListOrdered,
  code: Code,
  unlink: Unlink,
}

const props = withDefaults(defineProps<{
  name: BtgIconName
  size?: 'small' | 'medium' | 'large' | 'xl'
  decorative?: boolean
  label?: string
}>(), { size: 'medium', decorative: undefined, label: undefined })

const isDecorative = computed(() => props.decorative ?? !props.label)
</script>
