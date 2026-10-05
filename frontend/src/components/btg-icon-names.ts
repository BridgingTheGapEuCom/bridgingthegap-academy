export const btgIconNames = [
  'search', 'plus', 'add', 'edit', 'delete', 'trash', 'move', 'drag',
  'lesson', 'document', 'module', 'folder', 'image', 'text', 'callout', 'info',
  'divider', 'previous', 'arrowLeft', 'next', 'arrowRight', 'overflow', 'more',
  'settings', 'objective', 'target', 'content', 'layers', 'chevronDown', 'chevronRight',
] as const

export type BtgIconName = (typeof btgIconNames)[number]
