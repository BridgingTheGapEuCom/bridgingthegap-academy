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
  const draft = computed(() => {
    if (!context.draft.value) throw new Error('Authoring draft context is unavailable')
    return context.draft.value
  })
  return { ...context, draft }
}
