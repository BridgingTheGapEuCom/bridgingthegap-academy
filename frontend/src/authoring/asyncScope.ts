import { onBeforeUnmount, watch } from 'vue'
import { useAuth } from '../auth/auth'

// A response belongs to the resource and session that dispatched it, including
// errors. An old hidden 404 must not hide a newly opened workspace.
export function useAuthoringAsyncScope(resource: () => string) {
  const auth = useAuth()
  let active = true
  let resourceVersion = 0
  const resourceKey = () => { try { return resource() } catch { return undefined } }
  watch(resourceKey, () => { resourceVersion += 1 }, { flush: 'sync' })
  onBeforeUnmount(() => { active = false })
  return () => {
    const session = auth.state.value
    const version = resourceVersion
    return () => {
      return active && session === auth.state.value && version === resourceVersion && resourceKey() !== undefined
    }
  }
}
