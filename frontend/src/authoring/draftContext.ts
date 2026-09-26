import { computed, inject, type ComputedRef, type InjectionKey, type Ref } from 'vue'
import type { AuthoringDraft } from './authoring'

export type AuthoringDraftContext = {
  draft: Readonly<Ref<AuthoringDraft | undefined>>
  replaceDraft: (draft: AuthoringDraft) => void
  markDraftUnavailable: () => void
}

export const authoringDraftContextKey: InjectionKey<AuthoringDraftContext> = Symbol('authoring-draft-context')

export function useAuthoringDraftContext(): AuthoringDraftContext & { draft: ComputedRef<AuthoringDraft> } {
  const context = inject(authoringDraftContextKey)
  if (!context) throw new Error('Authoring draft context is unavailable')
  let lastDraft = context.draft.value
  const draft = computed(() => {
    // Keep an unmounting child's getter stable while the shell removes private
    // context. New children are mounted only after the next Draft is loaded.
    if (context.draft.value) lastDraft = context.draft.value
    if (!lastDraft) throw new Error('Authoring draft context is unavailable')
    return lastDraft
  })
  return { ...context, draft }
}
