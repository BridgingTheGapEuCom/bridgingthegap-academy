<template>
  <li ref="item" class="authoring-members__item">
    <BtgPanel padding="compact">
      <div class="authoring-members__identity">
        <p class="authoring-members__user-id"><span>{{ labels.userId }}</span> <code>{{ member.userId }}</code></p>
        <div class="authoring-members__role"><span>{{ labels.currentRole }}</span> <BtgBadge>{{ labels.roleLabel(member.role) }}</BtgBadge></div>
      </div>

      <div class="authoring-members__controls">
        <BtgFormField :label="labels.roleFor(member.userId)" v-slot="{ controlId, describedBy }">
          <BtgSelect :id="controlId" v-model="selectedRole" :aria-describedby="describedBy" :disabled="busy || confirming">
            <option value="AUTHOR">{{ labels.author }}</option>
            <option value="MAINTAINER">{{ labels.maintainer }}</option>
          </BtgSelect>
        </BtgFormField>
        <BtgButton variant="secondary" :disabled="busy || confirming || selectedRole === member.role" @click="emit('change-role', { userId: member.userId, role: selectedRole })">{{ labels.saveRole }}</BtgButton>
        <template v-if="confirming">
          <p class="authoring-members__confirm" role="status">{{ labels.revokeQuestion(member.userId) }}</p>
          <BtgButton variant="danger" :disabled="busy" :aria-label="labels.confirmRevoke(member.userId)" @click="emit('revoke', member.userId)">{{ labels.confirmRevokeText }}</BtgButton>
          <BtgButton variant="secondary" :disabled="busy" @click="cancelRevoke">{{ labels.cancel }}</BtgButton>
        </template>
        <BtgButton v-else variant="danger-secondary" :disabled="busy" :aria-label="labels.revokeAccess(member.userId)" @click="confirmRevoke">{{ labels.revokeAccess(member.userId) }}</BtgButton>
      </div>
    </BtgPanel>
  </li>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { preserveFocusAfterRemoval } from '../authoring/focus'
import type { AuthoringActiveMember } from '../authoring/authoring'
import BtgBadge from './BtgBadge.vue'
import BtgButton from './BtgButton.vue'
import BtgFormField from './BtgFormField.vue'
import BtgPanel from './BtgPanel.vue'
import BtgSelect from './BtgSelect.vue'

export type AuthoringMemberItemLabels = {
  userId: string
  currentRole: string
  roleLabel: (role: AuthoringActiveMember['role']) => string
  roleFor: (userId: string) => string
  author: string
  maintainer: string
  saveRole: string
  revokeAccess: (userId: string) => string
  revokeQuestion: (userId: string) => string
  confirmRevoke: (userId: string) => string
  confirmRevokeText: string
  cancel: string
}

const props = defineProps<{ member: AuthoringActiveMember; busy: boolean; confirming: boolean; labels: AuthoringMemberItemLabels }>()
const emit = defineEmits<{
  'change-role': [value: { userId: string; role: AuthoringActiveMember['role'] }]
  'confirm-revoke': [userID: string]
  'cancel-revoke': []
  revoke: [userID: string]
}>()

const item = ref<HTMLElement>()
const selectedRole = ref<AuthoringActiveMember['role']>(props.member.role)
watch(() => props.member.role, (role) => { selectedRole.value = role })

function confirmRevoke() {
  const restoreFocus = preserveFocusAfterRemoval(() => item.value?.querySelector<HTMLElement>('.btg-button--danger-secondary'))
  emit('confirm-revoke', props.member.userId)
  void restoreFocus()
}
function cancelRevoke() {
  const restoreFocus = preserveFocusAfterRemoval(() => item.value?.querySelector<HTMLElement>('.btg-button--danger-secondary'))
  emit('cancel-revoke')
  void restoreFocus()
}

</script>
