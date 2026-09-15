import { computed, inject, type ComputedRef, type InjectionKey, type Ref } from 'vue'
import type { AuthoringDraft } from './authoring'

export const authoringDraftContextKey: InjectionKey<Readonly<Ref<AuthoringDraft | undefined>>> = Symbol('authoring-draft-context')

export function useAuthoringDraftContext(): ComputedRef<AuthoringDraft> {
  const context = inject(authoringDraftContextKey)
  if (!context) throw new Error('Authoring draft context is unavailable')
  return computed(() => {
    if (!context.value) throw new Error('Authoring draft context is unavailable')
    return context.value
  })
}
