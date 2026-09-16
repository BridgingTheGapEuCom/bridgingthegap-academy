import { nextTick } from 'vue'

// Only repair focus when its element was removed. Leave a user's subsequent
// focus choice alone; saves and status announcements do not need focus.
export function preserveFocusAfterRemoval(fallback: () => HTMLElement | null | undefined) {
  const previous = document.activeElement
  return async () => {
    await nextTick()
    if (previous && !previous.isConnected && document.activeElement === document.body) fallback()?.focus()
  }
}
