<script setup lang="ts">
import type { CanonicalBlock } from '../authoring/contentEditor'
import BtgFormField from './BtgFormField.vue'
import AuthoringRichTextEditor from './AuthoringRichTextEditor.vue'
import AuthoringInlineTextEditor from './AuthoringInlineTextEditor.vue'
import AuthoringAssetAttachment from './AuthoringAssetAttachment.vue'
const props = defineProps<{ block: CanonicalBlock; position: number; draftId: string; lessonId: string }>()
const emit = defineEmits<{ update: [block: CanonicalBlock]; unavailable: [] }>()
function field(name: 'code' | 'language' | 'title' | 'text' | 'attribution' | 'sourceUrl' | 'altText' | 'caption' | 'label' | 'description' | 'transcript', event: Event) {
  const value = (event.target as HTMLInputElement).value
  if (props.block.type === 'CODE' || props.block.type === 'QUOTE' || props.block.type === 'CALLOUT') {
    const payload = { ...props.block.payload, [name]: value }
    if (!value && name !== 'code' && name !== 'text') delete payload[name as keyof typeof payload]
    emit('update', { ...props.block, payload } as CanonicalBlock)
  } else if (props.block.type === 'IMAGE' && (name === 'altText' || name === 'caption')) {
    const payload = { ...props.block.payload, [name]: value }
    if (!value) delete payload[name]
    emit('update', { ...props.block, payload })
  } else if ((props.block.type === 'VIDEO' || props.block.type === 'AUDIO') && (name === 'title' || name === 'transcript')) {
    const payload = { ...props.block.payload, [name]: value }
    if (!value && name === 'transcript') delete payload.transcript
    emit('update', { ...props.block, payload } as CanonicalBlock)
  } else if (props.block.type === 'DOWNLOAD' && (name === 'label' || name === 'description')) {
    const payload = { ...props.block.payload, [name]: value }
    if (!value && name === 'description') delete payload.description
    emit('update', { ...props.block, payload })
  }
}
function imageDecorative(event: Event) {
  if (props.block.type !== 'IMAGE') return
  const decorative = (event.target as HTMLInputElement).checked
  emit('update', { ...props.block, payload: { ...props.block.payload, decorative, ...(decorative ? { altText: '' } : {}) } })
}
function attach(field: 'asset' | 'captionsAsset', assetKey: string) {
  if (props.block.type === 'IMAGE' || props.block.type === 'AUDIO' || props.block.type === 'DOWNLOAD') {
    if (field === 'asset') emit('update', { ...props.block, payload: { ...props.block.payload, asset: { assetKey } } } as CanonicalBlock)
    return
  }
  if (props.block.type === 'VIDEO') emit('update', { ...props.block, payload: { ...props.block.payload, [field]: { assetKey } } } as CanonicalBlock)
}
</script>

<template>
  <AuthoringRichTextEditor v-if="block.type === 'TEXT'" :content="block.payload.content" :label="`Block ${position} text`" @update:content="emit('update', { ...block, payload: { ...block.payload, content: $event } })" />
  <template v-else-if="block.type === 'HEADING'">
    <BtgFormField label="Heading level" v-slot="{ controlId }">
      <select :id="controlId" :value="block.payload.level" @change="emit('update', { ...block, payload: { ...block.payload, level: Number(($event.target as HTMLSelectElement).value) } })"><option :value="2">Heading 2</option><option :value="3">Heading 3</option><option :value="4">Heading 4</option></select>
    </BtgFormField>
    <AuthoringInlineTextEditor :items="block.payload.content" :label="`Block ${position} heading`" @update:items="emit('update', { ...block, payload: { ...block.payload, content: $event } })" />
  </template>
  <template v-else-if="block.type === 'CODE'">
    <BtgFormField label="Code" :error="!block.payload.code ? 'Enter code to display.' : undefined" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" :value="block.payload.code" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" spellcheck="false" maxlength="100000" required @input="field('code', $event)" /></BtgFormField>
    <BtgFormField label="Code language" v-slot="{ controlId }"><input :id="controlId" :value="block.payload.language" maxlength="64" @input="field('language', $event)" /></BtgFormField>
    <BtgFormField label="Code title" v-slot="{ controlId }"><input :id="controlId" :value="block.payload.title" maxlength="240" @input="field('title', $event)" /></BtgFormField>
  </template>
  <template v-else-if="block.type === 'QUOTE'">
    <BtgFormField label="Quote text" :error="!block.payload.text.trim() ? 'Enter quote text.' : undefined" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" :value="block.payload.text" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" maxlength="50000" required @input="field('text', $event)" /></BtgFormField>
    <BtgFormField label="Quote attribution" v-slot="{ controlId }"><input :id="controlId" :value="block.payload.attribution" maxlength="240" @input="field('attribution', $event)" /></BtgFormField>
    <BtgFormField label="Quote source URL" v-slot="{ controlId }"><input :id="controlId" :value="block.payload.sourceUrl" maxlength="2048" @input="field('sourceUrl', $event)" /></BtgFormField>
  </template>
  <template v-else-if="block.type === 'CALLOUT'">
    <BtgFormField label="Callout kind" v-slot="{ controlId }"><select :id="controlId" :value="block.payload.kind" @change="emit('update', { ...block, payload: { ...block.payload, kind: ($event.target as HTMLSelectElement).value as 'INFO' | 'NOTE' | 'WARNING' | 'TIP' } })"><option value="INFO">Information</option><option value="NOTE">Note</option><option value="WARNING">Warning</option><option value="TIP">Tip</option></select></BtgFormField>
    <BtgFormField label="Callout title" v-slot="{ controlId }"><input :id="controlId" :value="block.payload.title" maxlength="240" @input="field('title', $event)" /></BtgFormField>
    <AuthoringRichTextEditor :content="block.payload.content" :label="`Block ${position} callout`" @update:content="emit('update', { ...block, payload: { ...block.payload, content: $event } })" />
  </template>
  <template v-else-if="block.type === 'IMAGE'">
    <AuthoringAssetAttachment :draft-id="draftId" :lesson-id="lessonId" :block-key="block.key" type="IMAGE" label="Image" :current-asset-key="block.payload.asset.assetKey" @attached="attach('asset', $event.assetKey)" @unavailable="emit('unavailable')" />
    <BtgFormField label="Alternative text" :required="!block.payload.decorative" :error="!block.payload.decorative && !block.payload.altText?.trim() ? 'Describe this image, or mark it decorative.' : undefined" v-slot="{ controlId, describedBy, invalid }"><input :id="controlId" :value="block.payload.altText" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" maxlength="1000" :required="!block.payload.decorative" :disabled="block.payload.decorative" @input="field('altText', $event)" /></BtgFormField>
    <label><input type="checkbox" :checked="block.payload.decorative" @change="imageDecorative" /> Decorative image</label>
    <BtgFormField label="Caption" v-slot="{ controlId }"><input :id="controlId" :value="block.payload.caption" maxlength="4000" @input="field('caption', $event)" /></BtgFormField>
  </template>
  <template v-else-if="block.type === 'VIDEO'">
    <AuthoringAssetAttachment :draft-id="draftId" :lesson-id="lessonId" :block-key="block.key" type="VIDEO" label="Video" :current-asset-key="block.payload.asset.assetKey" @attached="attach('asset', $event.assetKey)" @unavailable="emit('unavailable')" />
    <AuthoringAssetAttachment :draft-id="draftId" :lesson-id="lessonId" :block-key="block.key" type="DOWNLOAD" label="Captions" :current-asset-key="block.payload.captionsAsset.assetKey" @attached="attach('captionsAsset', $event.assetKey)" @unavailable="emit('unavailable')" />
    <BtgFormField label="Video title" :error="!block.payload.title.trim() ? 'Enter a video title.' : undefined" v-slot="{ controlId, describedBy, invalid }"><input :id="controlId" :value="block.payload.title" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" maxlength="240" required @input="field('title', $event)" /></BtgFormField>
    <BtgFormField label="Video transcript" :error="!block.payload.transcript?.trim() && !block.payload.transcriptAsset ? 'Provide a transcript or transcript asset.' : undefined" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" :value="block.payload.transcript" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" maxlength="50000" @input="field('transcript', $event)" /></BtgFormField>
  </template>
  <template v-else-if="block.type === 'AUDIO'">
    <AuthoringAssetAttachment :draft-id="draftId" :lesson-id="lessonId" :block-key="block.key" type="AUDIO" label="Audio" :current-asset-key="block.payload.asset.assetKey" @attached="attach('asset', $event.assetKey)" @unavailable="emit('unavailable')" />
    <BtgFormField label="Audio title" :error="!block.payload.title.trim() ? 'Enter an audio title.' : undefined" v-slot="{ controlId, describedBy, invalid }"><input :id="controlId" :value="block.payload.title" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" maxlength="240" required @input="field('title', $event)" /></BtgFormField>
    <BtgFormField label="Audio transcript" :error="!block.payload.transcript?.trim() && !block.payload.transcriptAsset ? 'Provide a transcript or transcript asset.' : undefined" v-slot="{ controlId, describedBy, invalid }"><textarea :id="controlId" :value="block.payload.transcript" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" maxlength="50000" @input="field('transcript', $event)" /></BtgFormField>
  </template>
  <template v-else-if="block.type === 'DOWNLOAD'">
    <AuthoringAssetAttachment :draft-id="draftId" :lesson-id="lessonId" :block-key="block.key" type="DOWNLOAD" label="Download" :current-asset-key="block.payload.asset.assetKey" @attached="attach('asset', $event.assetKey)" @unavailable="emit('unavailable')" />
    <BtgFormField label="Download label" :error="!block.payload.label.trim() ? 'Enter a download label.' : undefined" v-slot="{ controlId, describedBy, invalid }"><input :id="controlId" :value="block.payload.label" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" maxlength="240" required @input="field('label', $event)" /></BtgFormField>
    <BtgFormField label="Download description" v-slot="{ controlId }"><textarea :id="controlId" :value="block.payload.description" maxlength="4000" @input="field('description', $event)" /></BtgFormField>
  </template>
  <p v-else-if="block.type === 'DIVIDER'" class="authoring-section__intro">A semantic divider. No configuration is needed.</p>
  <template v-else>
    <p>This {{ block.type.toLowerCase().replaceAll('_', ' ') }} block is preserved read-only. You can move or remove it.</p>
    <details><summary>Preserved canonical payload</summary><pre class="authoring-content__payload">{{ JSON.stringify(block.payload, null, 2) }}</pre></details>
  </template>
</template>
