<template>
  <section class="authoring-section authoring-members" aria-labelledby="authoring-members-title">
    <header class="authoring-members__header">
      <h2 id="authoring-members-title" tabindex="-1">Members</h2>
      <p class="authoring-section__intro">Manage access with opaque user IDs. This Draft does not look up names or email addresses.</p>
    </header>

    <p v-if="state.kind === 'loading'" class="authoring-members__state" role="status">Loading current members…</p>
    <div v-else-if="state.kind === 'unavailable'" class="authoring-members__state">
      <p>We couldn’t load current members right now.</p>
      <BtgButton variant="secondary" @click="loadMembers()">Try again</BtgButton>
    </div>

    <template v-else>
      <p v-if="message" class="authoring-members__status" role="status">{{ message }}</p>
      <p v-if="formError" class="authoring-members__error" role="alert">{{ formError }}</p>
      <div v-if="conflict" class="authoring-members__conflict" role="status">
        <p>This membership change could not be completed. The Draft may have changed, or it must keep at least one maintainer. Your current form values are still here. Reload the latest Draft before trying again.</p>
        <BtgButton variant="secondary" :disabled="busy || reloading" @click="reloadLatest">{{ reloading ? 'Reloading…' : 'Reload latest Draft' }}</BtgButton>
      </div>

      <form class="authoring-members__add" :aria-busy="busy" novalidate @submit.prevent="addMember">
        <h3>Add member</h3>
        <BtgFormField label="User ID" required description="Enter an existing opaque user ID. This is not an email address or name." :error="addErrors.userId" v-slot="{ controlId, describedBy, invalid }">
          <BtgTextInput :id="controlId" v-model="addForm.userId" :disabled="busy || reloading" :aria-describedby="describedBy" :invalid="invalid" autocomplete="off" required />
        </BtgFormField>
        <BtgFormField label="Role" required v-slot="{ controlId, describedBy }">
          <select :id="controlId" v-model="addForm.role" :aria-describedby="describedBy" :disabled="busy || reloading || conflict" required>
            <option value="AUTHOR">Author</option>
            <option value="MAINTAINER">Maintainer</option>
          </select>
        </BtgFormField>
        <BtgButton type="submit" :disabled="busy || reloading || conflict">{{ busy ? 'Saving…' : 'Add member' }}</BtgButton>
      </form>

      <section class="authoring-members__current" aria-labelledby="authoring-current-members-title">
        <h3 id="authoring-current-members-title">Current members</h3>
        <p v-if="!state.members.length" class="authoring-members__empty">No active members were returned for this Draft.</p>
        <ul v-else class="authoring-members__list" aria-label="Current Draft members">
          <AuthoringMemberItem
            v-for="member in state.members"
            :key="member.userId"
            :member="member"
            :busy="busy || reloading || conflict"
            :confirming="confirmingUserID === member.userId"
            @change-role="changeRole"
            @confirm-revoke="confirmingUserID = $event"
            @cancel-revoke="confirmingUserID = undefined"
            @revoke="revokeMember"
          />
        </ul>
      </section>
    </template>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, reactive, ref, watch } from 'vue'
import { APIProblemError } from '../api/client'
import { preserveFocusAfterRemoval } from '../authoring/focus'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import {
  addAuthoringMember,
  changeAuthoringMemberRole,
  getAuthoringDraft,
  getAuthoringDraftMembers,
  isAuthoringDraftID,
  revokeAuthoringMember,
  type AuthoringActiveMember,
} from '../authoring/authoring'
import { useAuthoringDraftContext } from '../authoring/draftContext'
import AuthoringMemberItem from '../components/AuthoringMemberItem.vue'
import BtgButton from '../components/BtgButton.vue'
import BtgFormField from '../components/BtgFormField.vue'
import BtgTextInput from '../components/BtgTextInput.vue'

type State = { kind: 'loading' } | { kind: 'ready'; members: AuthoringActiveMember[] } | { kind: 'unavailable' }

const { draft, replaceDraft, markDraftUnavailable } = useAuthoringDraftContext()
const state = ref<State>({ kind: 'loading' })
const busy = ref(false)
const reloading = ref(false)
const conflict = ref(false)
const formError = ref<string>()
const message = ref<string>()
const confirmingUserID = ref<string>()
const addForm = reactive<{ userId: string; role: AuthoringActiveMember['role'] }>({ userId: '', role: 'AUTHOR' })
const addErrors = reactive<Record<string, string | undefined>>({})
let requestVersion = 0
let active = true

const captureScope = useAuthoringAsyncScope(() => draft.value.id)

watch(() => draft.value.id, () => { void loadMembers() }, { immediate: true })
onBeforeUnmount(() => { active = false })

function clearFeedback() {
  formError.value = undefined
  message.value = undefined
}

async function loadMembers(refresh = false) {
  const isCurrent = captureScope()
  const generation = ++requestVersion
  if (!refresh) state.value = { kind: 'loading' }
  try {
    const response = await getAuthoringDraftMembers(draft.value.id)
    if (!isCurrent()) return
    if (!active || generation !== requestVersion) return
    state.value = { kind: 'ready', members: response.members }
  } catch (error) {
    if (!isCurrent()) return
    if (!active || generation !== requestVersion) return
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
    state.value = { kind: 'unavailable' }
  }
}

async function reloadLatest() {
  const isCurrent = captureScope()
  if (busy.value || reloading.value) return
  reloading.value = true
  clearFeedback()
  try {
    const [latestDraft, memberships] = await Promise.all([getAuthoringDraft(draft.value.id), getAuthoringDraftMembers(draft.value.id)])
    if (!isCurrent()) return
    if (!active) return
    replaceDraft(latestDraft)
    state.value = { kind: 'ready', members: memberships.members }
    conflict.value = false
    confirmingUserID.value = undefined
    message.value = 'The latest Draft members have been loaded.'
  } catch (error) {
    if (!isCurrent()) return
    handleReadError(error, 'We couldn’t reload this Draft right now. Please try again.')
  } finally {
    if (isCurrent()) reloading.value = false
  }
}

async function addMember() {
  for (const key of Object.keys(addErrors)) delete addErrors[key]
  if (!isAuthoringDraftID(addForm.userId)) addErrors.userId = 'Enter a valid user ID.'
  if (Object.keys(addErrors).length || busy.value || conflict.value) return
  await mutate(async () => {
    const isCurrent = captureScope()
    const result = await addAuthoringMember(draft.value.id, {
      userId: addForm.userId,
      role: addForm.role,
      expectedDraftRevision: draft.value.revision,
    })
    if (!isCurrent()) return
    addForm.userId = ''
    addForm.role = 'AUTHOR'
    await commit(result.draftRevision, 'Member added.')
  })
}

async function changeRole(input: { userId: string; role: AuthoringActiveMember['role'] }) {
  await mutate(async () => {
    const isCurrent = captureScope()
    const result = await changeAuthoringMemberRole(draft.value.id, input.userId, {
      role: input.role,
      expectedDraftRevision: draft.value.revision,
    })
    if (!isCurrent()) return
    await commit(result.draftRevision, 'Member role updated.')
  })
}

async function revokeMember(userID: string) {
  await mutate(async () => {
    const isCurrent = captureScope()
    const result = await revokeAuthoringMember(draft.value.id, userID, draft.value.revision)
    if (!isCurrent()) return
    confirmingUserID.value = undefined
    await commit(result.draftRevision, 'Member access revoked.')
  })
}

async function mutate(operation: () => Promise<void>) {
  const isCurrent = captureScope()
  if (busy.value || reloading.value || conflict.value) return
  const restoreFocus = preserveFocusAfterRemoval(() => document.getElementById('authoring-members-title'))
  busy.value = true
  clearFeedback()
  try {
    await operation()
    if (!isCurrent()) return
    void restoreFocus()
  } catch (error) {
    if (!isCurrent()) return
    if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
    if (error instanceof APIProblemError && error.status === 409) { conflict.value = true; return }
    formError.value = error instanceof APIProblemError && error.status === 400
      ? 'We couldn’t save this membership change. Check the values and try again.'
      : 'We couldn’t save this membership change right now. Please try again.'
  } finally {
    if (isCurrent()) busy.value = false
  }
}

async function commit(revision: number, successMessage: string) {
  const isCurrent = captureScope()
  replaceDraft({ ...draft.value, revision })
  await loadMembers(true)
    if (!isCurrent()) return
  message.value = successMessage
}

function handleReadError(error: unknown, fallback: string) {
  if (error instanceof APIProblemError && error.status === 404) { markDraftUnavailable(); return }
  formError.value = fallback
}
</script>
