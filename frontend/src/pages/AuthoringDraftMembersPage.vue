<template>
  <section class="authoring-section authoring-members" aria-labelledby="authoring-members-title">
    <AuthoringPageTitle id="authoring-members-title" :title="t('authoring.members.title')" :description="t('authoring.members.description')" focusable />

    <BtgPanel v-if="state.kind === 'loading'" variant="subtle" padding="compact" class="authoring-members__state"><p role="status">{{ t('authoring.members.loading') }}</p></BtgPanel>
    <BtgPanel v-else-if="state.kind === 'unavailable'" variant="subtle" padding="compact" class="authoring-members__state">
      <p>{{ t('authoring.members.unavailable') }}</p>
      <BtgButton variant="secondary" @click="loadMembers()">{{ t('authoring.members.tryAgain') }}</BtgButton>
    </BtgPanel>

    <template v-else>
      <BtgPanel v-if="message" variant="subtle" padding="compact" class="authoring-members__status"><p role="status">{{ message }}</p></BtgPanel>
      <BtgPanel v-if="formError" variant="subtle" padding="compact" class="authoring-members__error"><p role="alert">{{ formError }}</p></BtgPanel>
      <BtgPanel v-if="conflict" variant="subtle" padding="compact" class="authoring-members__conflict" role="status">
        <p>{{ t('authoring.members.changedElsewhere') }}</p>
        <BtgButton variant="secondary" :disabled="busy || reloading" @click="reloadLatest">{{ reloading ? t('authoring.members.reloading') : t('authoring.members.reloadLatest') }}</BtgButton>
      </BtgPanel>

      <form class="authoring-members__add" :aria-busy="busy" novalidate @submit.prevent="addMember">
        <AuthoringSection heading-id="authoring-add-member-title" :title="t('authoring.members.add')">
          <BtgFormField :label="t('authoring.members.userId')" required :description="t('authoring.members.userIdHelp')" :error="addErrors.userId" v-slot="{ controlId, describedBy, invalid }">
            <BtgTextInput :id="controlId" v-model="addForm.userId" :disabled="busy || reloading" :aria-describedby="describedBy" :invalid="invalid" autocomplete="off" required />
          </BtgFormField>
          <BtgFormField :label="t('authoring.members.role')" required v-slot="{ controlId, describedBy }">
            <BtgSelect :id="controlId" v-model="addForm.role" :aria-describedby="describedBy" :disabled="busy || reloading || conflict" required>
              <option value="AUTHOR">{{ t('authoring.members.author') }}</option>
              <option value="MAINTAINER">{{ t('authoring.members.maintainer') }}</option>
            </BtgSelect>
          </BtgFormField>
          <BtgButton type="submit" :disabled="busy || reloading || conflict">{{ busy ? t('authoring.members.saving') : t('authoring.members.add') }}</BtgButton>
        </AuthoringSection>
      </form>

      <AuthoringSection heading-id="authoring-current-members-title" :title="t('authoring.members.current')" :description="t('authoring.members.currentDescription')">
        <p v-if="!state.members.length" class="authoring-members__empty">{{ t('authoring.members.empty') }}</p>
        <ul v-else class="authoring-members__list" :aria-label="t('authoring.members.list')">
          <AuthoringMemberItem
            v-for="member in state.members"
            :key="member.userId"
            :member="member"
            :busy="busy || reloading || conflict"
            :confirming="confirmingUserID === member.userId"
            :labels="memberItemLabels"
            @change-role="changeRole"
            @confirm-revoke="confirmingUserID = $event"
            @cancel-revoke="confirmingUserID = undefined"
            @revoke="revokeMember"
          />
        </ul>
      </AuthoringSection>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
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
import BtgPanel from '../components/BtgPanel.vue'
import BtgSelect from '../components/BtgSelect.vue'
import BtgTextInput from '../components/BtgTextInput.vue'
import AuthoringPageTitle from '../components/AuthoringPageTitle.vue'
import AuthoringSection from '../components/AuthoringSection.vue'

type State = { kind: 'loading' } | { kind: 'ready'; members: AuthoringActiveMember[] } | { kind: 'unavailable' }

const { draft, replaceDraft, markDraftUnavailable } = useAuthoringDraftContext()
const { t } = useI18n()
const state = ref<State>({ kind: 'loading' })
const busy = ref(false)
const reloading = ref(false)
const conflict = ref(false)
const formError = ref<string>()
const message = ref<string>()
const confirmingUserID = ref<string>()
const addForm = reactive<{ userId: string; role: AuthoringActiveMember['role'] }>({ userId: '', role: 'AUTHOR' })
const addErrors = reactive<Record<string, string | undefined>>({})
const memberItemLabels = computed(() => ({
  userId: t('authoring.members.userId'),
  currentRole: t('authoring.members.currentRole'),
  roleLabel: (role: AuthoringActiveMember['role']) => role === 'MAINTAINER' ? t('authoring.members.maintainer') : t('authoring.members.author'),
  roleFor: (userId: string) => t('authoring.members.roleFor', { userId }),
  author: t('authoring.members.author'),
  maintainer: t('authoring.members.maintainer'),
  saveRole: t('authoring.members.saveRole'),
  revokeAccess: (userId: string) => t('authoring.members.revokeAccess', { userId }),
  revokeQuestion: (userId: string) => t('authoring.members.revokeQuestion', { userId }),
  confirmRevoke: (userId: string) => t('authoring.members.confirmRevoke', { userId }),
  confirmRevokeText: t('authoring.members.confirmRevokeText'),
  cancel: t('authoring.members.cancel'),
}))
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
    message.value = t('authoring.members.latestLoaded')
  } catch (error) {
    if (!isCurrent()) return
    handleReadError(error, t('authoring.members.reloadUnavailable'))
  } finally {
    if (isCurrent()) reloading.value = false
  }
}

async function addMember() {
  for (const key of Object.keys(addErrors)) delete addErrors[key]
  if (!isAuthoringDraftID(addForm.userId)) addErrors.userId = t('authoring.members.validation.userId')
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
    await commit(result.draftRevision, t('authoring.members.added'))
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
    await commit(result.draftRevision, t('authoring.members.roleUpdated'))
  })
}

async function revokeMember(userID: string) {
  await mutate(async () => {
    const isCurrent = captureScope()
    const result = await revokeAuthoringMember(draft.value.id, userID, draft.value.revision)
    if (!isCurrent()) return
    confirmingUserID.value = undefined
    await commit(result.draftRevision, t('authoring.members.revoked'))
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
      ? t('authoring.members.saveInvalid')
      : t('authoring.members.saveUnavailable')
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
