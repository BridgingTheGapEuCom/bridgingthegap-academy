<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { APIProblemError } from '../api/client'
import { acceptsUploadedMedia, assetAcceptHint, assetTypeLabel, formatAssetByteSize, type AssetAttachmentType } from '../authoring/assets'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { listAuthoringDraftAssets, uploadAuthoringAsset, type AuthoringAsset, type AuthoringAssetSummary } from '../authoring/authoring'
import { draftAssetURL } from '../lesson/assets'
import BtgButton from './BtgButton.vue'
import BtgDialog from './BtgDialog.vue'
import BtgFormField from './BtgFormField.vue'
import BtgIcon from './BtgIcon.vue'
import BtgPanel from './BtgPanel.vue'

export interface AuthoringAssetAttachmentCopy {
  attachmentLabel: (label: string) => string
  attachedAssetFallback: string
  change: string
  choose: string
  pickerTitle: string
  pickerDescription: string
  uploadLabel: string
  uploadDescription: string
  selected: (filename: string) => string
  uploadAndUse: string
  uploading: string
  existingAssets: string
  loadingAssets: string
  loadUnavailable: string
  retry: string
  noAssets: string
  availableAssets: string
  useAsset: (filename: string) => string
  previous: string
  next: string
  cancel: string
  incompatibleUpload: string
  uploadedAttached: (filename: string) => string
  assetAttached: (filename: string) => string
  uploadInvalid: string
  uploadTooLarge: string
  sessionEnded: string
  uploadUnavailable: string
}

function legacyCopy(label: string, assetType: string): AuthoringAssetAttachmentCopy {
  return {
    attachmentLabel: (attachmentLabel) => `${attachmentLabel} attachment`,
    attachedAssetFallback: 'Attached asset',
    change: `Change ${label.toLowerCase()}`,
    choose: `Choose ${label.toLowerCase()}`,
    pickerTitle: `Change ${label.toLowerCase()}`,
    pickerDescription: `Choose an existing ${assetType} asset or upload a new one.`,
    uploadLabel: `Upload new ${label.toLowerCase()}`,
    uploadDescription: `Choose a file to upload for this ${assetType} block.`,
    selected: (filename) => `Selected: ${filename}`,
    uploadAndUse: 'Upload and use',
    uploading: 'Uploading…',
    existingAssets: `Existing ${assetType === 'image' ? 'images' : 'assets'}`,
    loadingAssets: 'Loading uploaded assets…',
    loadUnavailable: 'We couldn’t load uploaded assets right now. Try again.',
    retry: 'Retry',
    noAssets: 'No uploaded assets yet.',
    availableAssets: 'Available uploaded assets',
    useAsset: (filename) => `Use ${filename}`,
    previous: 'Previous',
    next: 'Next',
    cancel: 'Cancel',
    incompatibleUpload: `This uploaded file is not suitable for this ${assetType} block, so it was not attached.`,
    uploadedAttached: (filename) => `${filename} uploaded and attached. Save content to keep this reference.`,
    assetAttached: (filename) => `${filename} attached. Save content to keep this reference.`,
    uploadInvalid: 'We couldn’t upload that file. Choose a different file and try again.',
    uploadTooLarge: 'The selected file is too large. Choose a smaller file and try again.',
    sessionEnded: 'Your session has ended. Sign in to continue.',
    uploadUnavailable: 'We couldn’t upload this file right now. Please try again.',
  }
}

const props = defineProps<{
  draftId: string
  lessonId: string
  blockKey: string
  type: AssetAttachmentType
  label: string
  currentAssetKey: string
  copy?: AuthoringAssetAttachmentCopy
}>()
const emit = defineEmits<{ attached: [asset: AuthoringAsset]; unavailable: [] }>()
const selected = ref<File>()
const uploaded = ref<AuthoringAsset>()
const uploading = ref(false)
const error = ref<string>()
const message = ref<string>()
const fileInput = ref<HTMLInputElement>()
const pickerOpen = ref(false)
const listed = ref<AuthoringAssetSummary[]>([])
const listLimit = ref(20); const listOffset = ref(0); const listTotal = ref(0)
const listing = ref(false); const listError = ref<string>()
let listRefreshRequested = false
let uploadVersion = 0
let listVersion = 0
let active = true
const captureScope = useAuthoringAsyncScope(() => `${props.draftId}/${props.lessonId}/${props.blockKey}/${props.label}`)
const copy = computed(() => props.copy ?? legacyCopy(props.label, assetTypeLabel(props.type)))

watch(() => [props.draftId, props.lessonId, props.blockKey, props.label], () => {
  uploadVersion += 1
  uploading.value = false
  selected.value = undefined
  uploaded.value = undefined
  error.value = undefined
  message.value = undefined
  listed.value = []; listOffset.value = 0; listTotal.value = 0; listError.value = undefined
  if (fileInput.value) fileInput.value.value = ''
  void loadAssets(0)
}, { immediate: true })
watch(() => props.currentAssetKey, (assetKey) => {
  if (uploaded.value?.assetKey !== assetKey) uploaded.value = undefined
})
onBeforeUnmount(() => { active = false })

function chooseFile(event: Event) {
  selected.value = (event.target as HTMLInputElement).files?.[0] ?? undefined
  error.value = undefined
  message.value = undefined
}

function openPicker() { pickerOpen.value = true }

function closePicker() { pickerOpen.value = false }

function clearSelectedFile() {
  selected.value = undefined
  if (fileInput.value) fileInput.value.value = ''
}

async function upload() {
  const file = selected.value
  if (!file || uploading.value) return
  const isCurrent = captureScope()
  const generation = ++uploadVersion
  uploading.value = true
  error.value = undefined
  message.value = undefined
  try {
    const asset = await uploadAuthoringAsset(props.draftId, file)
    if (!isCurrent() || !active || generation !== uploadVersion) return
    if (!acceptsUploadedMedia(props.type, asset.mediaType)) {
      error.value = copy.value.incompatibleUpload
      return
    }
    uploaded.value = asset
    emit('attached', asset)
    clearSelectedFile()
    message.value = copy.value.uploadedAttached(asset.filename)
    refreshAssets()
    closePicker()
  } catch (reason) {
    if (!isCurrent() || !active || generation !== uploadVersion) return
    if (reason instanceof APIProblemError && reason.status === 404) {
      emit('unavailable')
      return
    }
    // Resetting the native value lets a user deliberately choose the same file
    // again after a failed request; browser file controls otherwise suppress it.
    clearSelectedFile()
    error.value = uploadError(reason)
  } finally {
    if (isCurrent() && active && generation === uploadVersion) uploading.value = false
  }
}

const compatibleAssets = computed(() => listed.value.filter((asset) => acceptsUploadedMedia(props.type, asset.mediaType)))
const currentAsset = computed(() => listed.value.find((asset) => asset.assetKey === props.currentAssetKey))
const currentImageURL = computed(() => props.type === 'IMAGE' && props.currentAssetKey
  ? draftAssetURL({ draftID: props.draftId }, { assetKey: props.currentAssetKey })
  : undefined)
async function loadAssets(offset = listOffset.value) {
  if (listing.value) return
  const isCurrent = captureScope(); const generation = ++listVersion; listing.value = true; listError.value = undefined
  try {
    const page = await listAuthoringDraftAssets(props.draftId, listLimit.value, offset)
    if (!isCurrent() || !active || generation !== listVersion) return
    listed.value = page.items; listLimit.value = page.limit; listOffset.value = page.offset; listTotal.value = page.total
  } catch (reason) {
    if (!isCurrent() || !active || generation !== listVersion) return
    if (reason instanceof APIProblemError && reason.status === 404) { emit('unavailable'); return }
    listError.value = copy.value.loadUnavailable
  } finally {
    if (isCurrent() && active && generation === listVersion) {
      listing.value = false
      if (listRefreshRequested) { listRefreshRequested = false; void loadAssets(0) }
    }
  }
}
function refreshAssets() { if (listing.value) { listRefreshRequested = true; return }; void loadAssets(0) }
function selectExisting(asset: AuthoringAssetSummary) {
  if (!acceptsUploadedMedia(props.type, asset.mediaType)) return
  emit('attached', { ...asset, status: 'AVAILABLE' })
  message.value = copy.value.assetAttached(asset.filename)
  closePicker()
}

function uploadError(reason: unknown): string {
  if (reason instanceof APIProblemError) {
    if (reason.status === 400) return copy.value.uploadInvalid
    if (reason.status === 413) return copy.value.uploadTooLarge
    if (reason.status === 401) return copy.value.sessionEnded
  }
  return copy.value.uploadUnavailable
}
</script>

<template>
  <section class="authoring-asset-attachment" :class="{ 'authoring-asset-attachment--image': type === 'IMAGE' }" :aria-busy="uploading" :aria-label="copy.attachmentLabel(label)">
    <div v-if="currentAssetKey" class="authoring-asset-attachment__current">
      <img v-if="currentImageURL" :src="currentImageURL" alt="" />
      <div class="authoring-asset-attachment__field">
        <p class="authoring-asset-attachment__label">{{ label }}</p>
        <BtgPanel class="authoring-asset-attachment__summary" padding="compact">
          <BtgIcon :name="type === 'IMAGE' ? 'image' : 'document'" decorative />
          <div><strong>{{ currentAsset?.filename ?? uploaded?.filename ?? copy.attachedAssetFallback }}</strong><small v-if="currentAsset">{{ currentAsset.mediaType }} · {{ formatAssetByteSize(currentAsset.byteSize) }}</small></div>
          <BtgButton variant="secondary" @click="openPicker">{{ copy.change }}</BtgButton>
        </BtgPanel>
      </div>
    </div>
    <BtgButton v-else variant="secondary" @click="openPicker">{{ copy.choose }}</BtgButton>
    <slot />
    <BtgDialog v-model:open="pickerOpen" :title="copy.pickerTitle" :description="copy.pickerDescription" :dismiss-label="copy.cancel" :show-trigger="false">
      <div class="authoring-asset-attachment__picker-content">
      <BtgFormField :label="copy.uploadLabel" :description="copy.uploadDescription" v-slot="{ controlId, describedBy }">
        <input :id="controlId" ref="fileInput" type="file" :accept="assetAcceptHint(type)" :aria-describedby="describedBy" :disabled="uploading" @change="chooseFile" />
      </BtgFormField>
      <div class="authoring-asset-attachment__upload"><p v-if="selected" class="authoring-asset-attachment__selection">{{ copy.selected(selected.name) }}</p><BtgButton :disabled="!selected || uploading" :loading="uploading" @click="upload">{{ copy.uploadAndUse }}</BtgButton></div>
      <p v-if="error" class="authoring-asset-attachment__error" role="alert">{{ error }}</p>
      <section class="authoring-asset-attachment__existing" aria-labelledby="`asset-picker-existing-${blockKey}`">
      <h4 :id="`asset-picker-existing-${blockKey}`">{{ copy.existingAssets }}</h4>
      <p v-if="listing" role="status">{{ copy.loadingAssets }}</p>
      <p v-else-if="listError" role="alert">{{ listError }} <BtgButton variant="secondary" @click="refreshAssets">{{ copy.retry }}</BtgButton></p>
      <p v-else-if="!compatibleAssets.length">{{ copy.noAssets }}</p>
      <ul v-else class="authoring-asset-attachment__list" :aria-label="copy.availableAssets"><li v-for="asset in compatibleAssets" :key="asset.assetKey"><BtgPanel padding="compact"><BtgButton class="authoring-asset-attachment__asset-choice" variant="tertiary" :aria-label="copy.useAsset(asset.filename)" @click="selectExisting(asset)"><img v-if="type === 'IMAGE'" :src="draftAssetURL({ draftID: draftId }, { assetKey: asset.assetKey })" alt="" /><span><strong>{{ asset.filename }}</strong><small>{{ asset.mediaType }} · {{ formatAssetByteSize(asset.byteSize) }}</small></span></BtgButton></BtgPanel></li></ul>
      <p v-if="listTotal > listLimit"><BtgButton variant="secondary" :disabled="listOffset === 0 || listing" @click="loadAssets(Math.max(0, listOffset - listLimit))">{{ copy.previous }}</BtgButton><BtgButton variant="secondary" :disabled="listOffset + listLimit >= listTotal || listing" @click="loadAssets(listOffset + listLimit)">{{ copy.next }}</BtgButton></p>
      </section>
      </div>
      <template #footer><BtgButton variant="secondary" @click="closePicker">{{ copy.cancel }}</BtgButton></template>
    </BtgDialog>
    <p v-if="message" class="authoring-asset-attachment__status" role="status">{{ message }}</p>
  </section>
</template>
