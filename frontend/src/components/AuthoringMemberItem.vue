<template>
  <li ref="item" class="authoring-members__item">
    <div class="authoring-members__identity">
      <p class="authoring-members__user-id"><span>User ID</span> <code>{{ member.userId }}</code></p>
      <p class="authoring-members__role">Current role: <strong>{{ roleLabel(member.role) }}</strong></p>
    </div>

    <div class="authoring-members__controls">
      <BtgFormField :label="`Role for ${member.userId}`" v-slot="{ controlId, describedBy }">
        <select :id="controlId" v-model="selectedRole" :aria-describedby="describedBy" :disabled="busy || confirming">
          <option value="AUTHOR">Author</option>
          <option value="MAINTAINER">Maintainer</option>
        </select>
      </BtgFormField>
      <BtgButton variant="secondary" :disabled="busy || confirming || selectedRole === member.role" @click="emit('change-role', { userId: member.userId, role: selectedRole })">Save role</BtgButton>
      <template v-if="confirming">
        <p class="authoring-members__confirm" role="status">Revoke access for <code>{{ member.userId }}</code>?</p>
        <BtgButton variant="destructive" :disabled="busy" :aria-label="`Confirm revoke access for ${member.userId}`" @click="emit('revoke', member.userId)">Confirm revoke</BtgButton>
        <BtgButton variant="secondary" :disabled="busy" @click="cancelRevoke">Cancel</BtgButton>
      </template>
      <BtgButton v-else variant="destructive" :disabled="busy" :aria-label="`Revoke access for ${member.userId}`" @click="confirmRevoke">Revoke access</BtgButton>
    </div>
  </li>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { preserveFocusAfterRemoval } from '../authoring/focus'
import type { AuthoringActiveMember } from '../authoring/authoring'
import BtgButton from './BtgButton.vue'
import BtgFormField from './BtgFormField.vue'

const props = defineProps<{ member: AuthoringActiveMember; busy: boolean; confirming: boolean }>()
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
  const restoreFocus = preserveFocusAfterRemoval(() => item.value?.querySelector<HTMLElement>('.btg-button--destructive'))
  emit('confirm-revoke', props.member.userId)
  void restoreFocus()
}
function cancelRevoke() {
  const restoreFocus = preserveFocusAfterRemoval(() => item.value?.querySelector<HTMLElement>('.btg-button--destructive'))
  emit('cancel-revoke')
  void restoreFocus()
}

function roleLabel(role: AuthoringActiveMember['role']): string {
  return role === 'MAINTAINER' ? 'Maintainer' : 'Author'
}
</script>
