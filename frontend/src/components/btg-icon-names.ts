export const btgIconNames = [
  'search', 'plus', 'add', 'edit', 'delete', 'trash', 'move', 'drag',
  'lesson', 'document', 'module', 'folder', 'image', 'text', 'callout', 'info',
  'divider', 'video', 'audio', 'previous', 'arrowLeft', 'next', 'arrowRight', 'overflow', 'more',
  'settings', 'objective', 'target', 'content', 'layers', 'chevronDown', 'chevronRight',
  'bold', 'italic', 'link', 'list', 'listOrdered', 'code', 'unlink',
] as const

export type BtgIconName = (typeof btgIconNames)[number]
