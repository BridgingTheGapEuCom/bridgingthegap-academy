<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { APIProblemError } from '../api/client'
import { acceptsUploadedMedia, assetAcceptHint, assetTypeLabel, formatAssetByteSize, type AssetAttachmentType } from '../authoring/assets'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { listAuthoringDraftAssets, uploadAuthoringAsset, type AuthoringAsset, type AuthoringAssetSummary } from '../authoring/authoring'
import BtgButton from './BtgButton.vue'
import BtgFormField from './BtgFormField.vue'

const props = defineProps<{
  draftId: string
  lessonId: string
  blockKey: string
  type: AssetAttachmentType
  label: string
  currentAssetKey: string
}>()
const emit = defineEmits<{ attached: [asset: AuthoringAsset]; unavailable: [] }>()
const selected = ref<File>()
const uploaded = ref<AuthoringAsset>()
const uploading = ref(false)
const error = ref<string>()
const message = ref<string>()
const fileInput = ref<HTMLInputElement>()
const listed = ref<AuthoringAssetSummary[]>([])
const listLimit = ref(20); const listOffset = ref(0); const listTotal = ref(0)
const listing = ref(false); const listError = ref<string>()
let listRefreshRequested = false
let uploadVersion = 0
let listVersion = 0
let active = true
const captureScope = useAuthoringAsyncScope(() => `${props.draftId}/${props.lessonId}/${props.blockKey}/${props.label}`)

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
      error.value = `This uploaded file is not suitable for this ${assetTypeLabel(props.type)} block, so it was not attached.`
      return
    }
    uploaded.value = asset
    emit('attached', asset)
    clearSelectedFile()
    message.value = `${asset.filename} uploaded and attached. Save Lesson content to keep this reference.`
    refreshAssets()
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
    listError.value = 'We couldn’t load uploaded assets right now. Try again.'
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
  message.value = `${asset.filename} attached. Save Lesson content to keep this reference.`
}

function uploadError(reason: unknown): string {
  if (reason instanceof APIProblemError) {
    if (reason.status === 400) return 'We couldn’t upload that file. Choose a different file and try again.'
    if (reason.status === 413) return 'The selected file is too large. Choose a smaller file and try again.'
    if (reason.status === 401) return 'Your session has ended. Sign in to continue.'
  }
  return 'We couldn’t upload this file right now. Please try again.'
}
</script>

<template>
  <section class="authoring-asset-attachment" :aria-busy="uploading" :aria-label="`${label} attachment`">
    <p v-if="currentAssetKey" class="authoring-asset-attachment__current">An asset is attached.</p>
    <BtgFormField :label="`${label} file`" :description="`Choose a file to upload for this ${assetTypeLabel(type)} block.`" v-slot="{ controlId, describedBy }">
      <input :id="controlId" ref="fileInput" type="file" :accept="assetAcceptHint(type)" :aria-describedby="describedBy" :disabled="uploading" @change="chooseFile" />
    </BtgFormField>
    <p v-if="selected" class="authoring-asset-attachment__selection">Selected: {{ selected.name }}</p>
    <BtgButton variant="secondary" :disabled="!selected || uploading" @click="upload">{{ uploading ? 'Uploading…' : currentAssetKey ? 'Upload replacement' : 'Upload file' }}</BtgButton>
    <p v-if="uploaded" class="authoring-asset-attachment__metadata">Attached: {{ uploaded.filename }} · {{ uploaded.mediaType }} · {{ formatAssetByteSize(uploaded.byteSize) }}</p>
    <p v-if="error" class="authoring-asset-attachment__error" role="alert">{{ error }}</p>
    <p v-if="message" class="authoring-asset-attachment__status" role="status">{{ message }}</p>
    <details class="authoring-asset-attachment__existing">
      <summary>Choose an uploaded asset</summary>
      <p v-if="listing" role="status">Loading uploaded assets…</p>
      <p v-else-if="listError" role="alert">{{ listError }} <BtgButton variant="secondary" @click="refreshAssets">Retry</BtgButton></p>
      <p v-else-if="!compatibleAssets.length">No uploaded assets yet.</p>
      <ul v-else class="authoring-asset-attachment__list" aria-label="Available uploaded assets"><li v-for="asset in compatibleAssets" :key="asset.assetKey"><BtgButton variant="secondary" @click="selectExisting(asset)">Use {{ asset.filename }}</BtgButton><span>{{ asset.mediaType }} · {{ formatAssetByteSize(asset.byteSize) }} · {{ new Date(asset.createdAt).toLocaleDateString() }}</span></li></ul>
      <p v-if="listTotal > listLimit"><BtgButton variant="secondary" :disabled="listOffset === 0 || listing" @click="loadAssets(Math.max(0, listOffset - listLimit))">Previous</BtgButton><BtgButton variant="secondary" :disabled="listOffset + listLimit >= listTotal || listing" @click="loadAssets(listOffset + listLimit)">Next</BtgButton></p>
    </details>
  </section>
</template>
