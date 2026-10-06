<script setup lang="ts">
import { computed } from 'vue'
import type { CanonicalBlock } from '../authoring/contentEditor'
import type { AuthoringCourseWidget } from '../authoring/authoring'
import BtgFormField from './BtgFormField.vue'
import WidgetConfigurationForm from './WidgetConfigurationForm.vue'
import { configurationErrors, configurationInitial, type WidgetConfiguration } from '../plugins/configuration'
import AuthoringRichTextEditor from './AuthoringRichTextEditor.vue'
import AuthoringInlineTextEditor from './AuthoringInlineTextEditor.vue'
import AuthoringAssetAttachment, { type AuthoringAssetAttachmentCopy } from './AuthoringAssetAttachment.vue'
import AuthoringCheckboxField from './AuthoringCheckboxField.vue'
import AuthoringAssessmentAttachment, { type AuthoringAssessmentAttachmentCopy } from './AuthoringAssessmentAttachment.vue'
import BtgTextInput from './BtgTextInput.vue'
import BtgTextarea from './BtgTextarea.vue'
import BtgSelect from './BtgSelect.vue'
import { useI18n } from 'vue-i18n'
import { useApplicationFormatters } from '../i18n/formatters'
const props = defineProps<{ block: CanonicalBlock; position: number; draftId: string; lessonId: string; courseWidgets?: AuthoringCourseWidget[] }>()
const emit = defineEmits<{ update: [block: CanonicalBlock]; unavailable: [] }>()
const { t } = useI18n()
const { date } = useApplicationFormatters()
const widgetConfigurationCopy = computed(() => ({
  selectOption: t('authoring.content.widget.selectOption'),
  enabled: t('authoring.content.widget.enabled'),
  disabled: t('authoring.content.widget.disabled'),
}))
const assessmentCopy = computed<AuthoringAssessmentAttachmentCopy>(() => ({
  title: t('authoring.content.assessment.title'),
  selected: t('authoring.content.assessment.selected'),
  selectedUnavailable: t('authoring.content.assessment.selectedUnavailable'),
  loading: t('authoring.content.assessment.loading'),
  loadUnavailable: t('authoring.content.assessment.loadUnavailable'),
  retry: t('authoring.content.assessment.retry'),
  empty: t('authoring.content.assessment.empty'),
  available: t('authoring.content.assessment.available'),
  use: (title) => t('authoring.content.assessment.use', { title }),
  questions: (count) => t('authoring.content.assessment.questions', count),
  updated: (value) => t('authoring.content.assessment.updated', { date: date(value) }),
  previous: t('authoring.content.assessment.previous'),
  next: t('authoring.content.assessment.next'),
  pagination: (from, to, total) => t('authoring.content.assessment.pagination', { from, to, total }),
}))
const imageAttachmentCopy = computed<AuthoringAssetAttachmentCopy>(() => ({
  attachmentLabel: (label) => t('authoring.content.image.attachment', { label }),
  attachedAssetFallback: t('authoring.content.image.attachedAsset'),
  change: t('authoring.content.image.change'),
  choose: t('authoring.content.image.choose'),
  pickerTitle: t('authoring.content.image.pickerTitle'),
  pickerDescription: t('authoring.content.image.pickerDescription'),
  uploadLabel: t('authoring.content.image.uploadLabel'),
  uploadDescription: t('authoring.content.image.uploadDescription'),
  selected: (filename) => t('authoring.content.image.selected', { filename }),
  uploadAndUse: t('authoring.content.image.uploadAndUse'),
  uploading: t('authoring.content.image.uploading'),
  existingAssets: t('authoring.content.image.existingAssets'),
  loadingAssets: t('authoring.content.image.loadingAssets'),
  loadUnavailable: t('authoring.content.image.loadUnavailable'),
  retry: t('authoring.content.image.retry'),
  noAssets: t('authoring.content.image.noAssets'),
  availableAssets: t('authoring.content.image.availableAssets'),
  useAsset: (filename) => t('authoring.content.image.useAsset', { filename }),
  previous: t('authoring.content.image.previous'),
  next: t('authoring.content.image.next'),
  cancel: t('authoring.content.image.cancel'),
  incompatibleUpload: t('authoring.content.image.incompatibleUpload'),
  uploadedAttached: (filename) => t('authoring.content.image.uploadedAttached', { filename }),
  assetAttached: (filename) => t('authoring.content.image.assetAttached', { filename }),
  uploadInvalid: t('authoring.content.image.uploadInvalid'),
  uploadTooLarge: t('authoring.content.image.uploadTooLarge'),
  sessionEnded: t('authoring.content.image.sessionEnded'),
  uploadUnavailable: t('authoring.content.image.uploadUnavailable'),
}))
function mediaAttachmentCopy(label: string, assetType: string): AuthoringAssetAttachmentCopy {
  return {
    attachmentLabel: (attachmentLabel) => t('authoring.content.media.attachmentLabel', { label: attachmentLabel }),
    attachedAssetFallback: t('authoring.content.media.attachedAsset'),
    change: t('authoring.content.media.change', { label }),
    choose: t('authoring.content.media.choose', { label }),
    pickerTitle: t('authoring.content.media.pickerTitle', { label }),
    pickerDescription: t('authoring.content.media.pickerDescription', { assetType }),
    uploadLabel: t('authoring.content.media.uploadLabel', { label }),
    uploadDescription: t('authoring.content.media.uploadDescription', { assetType }),
    selected: (filename) => t('authoring.content.media.selected', { filename }),
    uploadAndUse: t('authoring.content.media.uploadAndUse'),
    uploading: t('authoring.content.media.uploading'),
    existingAssets: t('authoring.content.media.existingAssets', { assetType }),
    loadingAssets: t('authoring.content.media.loadingAssets'),
    loadUnavailable: t('authoring.content.media.loadUnavailable'),
    retry: t('authoring.content.media.retry'),
    noAssets: t('authoring.content.media.noAssets'),
    availableAssets: t('authoring.content.media.availableAssets'),
    useAsset: (filename) => t('authoring.content.media.useAsset', { filename }),
    previous: t('authoring.content.media.previous'),
    next: t('authoring.content.media.next'),
    cancel: t('authoring.content.media.cancel'),
    incompatibleUpload: t('authoring.content.media.incompatibleUpload', { assetType }),
    uploadedAttached: (filename) => t('authoring.content.media.uploadedAttached', { filename }),
    assetAttached: (filename) => t('authoring.content.media.assetAttached', { filename }),
    uploadInvalid: t('authoring.content.media.uploadInvalid'),
    uploadTooLarge: t('authoring.content.media.uploadTooLarge'),
    sessionEnded: t('authoring.content.media.sessionEnded'),
    uploadUnavailable: t('authoring.content.media.uploadUnavailable'),
  }
}
function imageField(name: 'altText' | 'caption', value: string) {
  if (props.block.type !== 'IMAGE') return
  const payload = { ...props.block.payload, [name]: value }
  if (!value) delete payload[name]
  emit('update', { ...props.block, payload })
}
function simpleField(name: 'code' | 'language' | 'title' | 'text' | 'attribution' | 'sourceUrl', value: string) {
  if (props.block.type !== 'CODE' && props.block.type !== 'QUOTE' && props.block.type !== 'CALLOUT') return
  const payload = { ...props.block.payload, [name]: value }
  if (!value && name !== 'code' && name !== 'text') delete payload[name as keyof typeof payload]
  emit('update', { ...props.block, payload } as CanonicalBlock)
}
function mediaField(name: 'title' | 'transcript' | 'label' | 'description', value: string) {
  if (props.block.type === 'VIDEO' || props.block.type === 'AUDIO') {
    const payload = { ...props.block.payload, [name]: value }
    if (!value && name === 'transcript') delete payload.transcript
    emit('update', { ...props.block, payload } as CanonicalBlock)
  } else if (props.block.type === 'DOWNLOAD') {
    const payload = { ...props.block.payload, [name]: value }
    if (!value && name === 'description') delete payload.description
    emit('update', { ...props.block, payload })
  }
}
function headingLevel(value: string) {
  if (props.block.type === 'HEADING') emit('update', { ...props.block, payload: { ...props.block.payload, level: Number(value) } })
}
function calloutKind(value: string) {
  if (props.block.type === 'CALLOUT') emit('update', { ...props.block, payload: { ...props.block.payload, kind: value as 'INFO' | 'NOTE' | 'WARNING' | 'TIP' } })
}
function attach(field: 'asset' | 'captionsAsset', assetKey: string) {
  if (props.block.type === 'IMAGE' || props.block.type === 'AUDIO' || props.block.type === 'DOWNLOAD') {
    if (field === 'asset') emit('update', { ...props.block, payload: { ...props.block.payload, asset: { assetKey } } } as CanonicalBlock)
    return
  }
  if (props.block.type === 'VIDEO') emit('update', { ...props.block, payload: { ...props.block.payload, [field]: { assetKey } } } as CanonicalBlock)
}
function attachAssessment(assessmentKey: string) {
  if (props.block.type === 'KNOWLEDGE_CHECK') emit('update', { ...props.block, payload: { assessmentKey } })
}
function chooseWidgetValue(value: string) {
  if (props.block.type !== 'PLUGIN_WIDGET') return
  const selected = props.courseWidgets?.find((widget) => `${widget.pluginId}:${widget.pluginVersion}:${widget.artifactDigest}:${widget.widgetId}` === value)
  if (!selected) return
  emit('update', { ...props.block, payload: { pluginId: selected.pluginId, pluginVersion: selected.pluginVersion, artifactDigest: selected.artifactDigest, widgetId: selected.widgetId, widgetType: 'COURSE_WIDGET', configuration: configurationInitial(selected.configuration) } })
}
function widgetConfiguration(configuration: WidgetConfiguration) {
  if (props.block.type !== 'PLUGIN_WIDGET') return
  emit('update', { ...props.block, payload: { ...props.block.payload, configuration } })
}
function selectedWidget() {
  if (props.block.type !== 'PLUGIN_WIDGET') return undefined
  const payload = props.block.payload
  return props.courseWidgets?.find((widget) => widget.pluginId === payload.pluginId && widget.pluginVersion === payload.pluginVersion && widget.artifactDigest === payload.artifactDigest && widget.widgetId === payload.widgetId)
}
</script>

<template>
  <AuthoringRichTextEditor v-if="block.type === 'TEXT'" :content="block.payload.content" :label="t('authoring.content.blocks.TEXT.editorLabel', { position })" code-blocks @update:content="emit('update', { ...block, payload: { ...block.payload, content: $event } })" />
  <template v-else-if="block.type === 'HEADING'">
    <BtgFormField :label="t('authoring.content.blocks.HEADING.level')" v-slot="{ controlId }">
      <BtgSelect :id="controlId" :model-value="String(block.payload.level)" @update:model-value="headingLevel"><option value="2">{{ t('authoring.content.blocks.HEADING.levelTwo') }}</option><option value="3">{{ t('authoring.content.blocks.HEADING.levelThree') }}</option><option value="4">{{ t('authoring.content.blocks.HEADING.levelFour') }}</option></BtgSelect>
    </BtgFormField>
    <AuthoringInlineTextEditor :items="block.payload.content" :label="t('authoring.content.blocks.HEADING.editorLabel', { position })" :copy="{ textLabel: (index) => t('authoring.content.blocks.HEADING.textLabel', { index }), preservedFormatting: t('authoring.content.blocks.HEADING.preservedFormatting'), textRequired: t('authoring.content.blocks.HEADING.textRequired'), hardBreakPreserved: t('authoring.content.blocks.HEADING.hardBreakPreserved') }" @update:items="emit('update', { ...block, payload: { ...block.payload, content: $event } })" />
  </template>
  <template v-else-if="block.type === 'CODE'">
    <BtgFormField :label="t('authoring.content.blocks.CODE.code')" :error="!block.payload.code ? t('authoring.content.blocks.CODE.codeRequired') : undefined" v-slot="{ controlId, describedBy, invalid }"><BtgTextarea :id="controlId" :model-value="block.payload.code" :aria-describedby="describedBy" :invalid="invalid" spellcheck="false" maxlength="100000" required @update:model-value="simpleField('code', $event)" /></BtgFormField>
    <BtgFormField :label="t('authoring.content.blocks.CODE.language')" v-slot="{ controlId }"><BtgTextInput :id="controlId" :model-value="block.payload.language" maxlength="64" @update:model-value="simpleField('language', $event)" /></BtgFormField>
    <BtgFormField :label="t('authoring.content.blocks.CODE.title')" v-slot="{ controlId }"><BtgTextInput :id="controlId" :model-value="block.payload.title" maxlength="240" @update:model-value="simpleField('title', $event)" /></BtgFormField>
  </template>
  <template v-else-if="block.type === 'QUOTE'">
    <BtgFormField :label="t('authoring.content.blocks.QUOTE.text')" :error="!block.payload.text.trim() ? t('authoring.content.blocks.QUOTE.textRequired') : undefined" v-slot="{ controlId, describedBy, invalid }"><BtgTextarea :id="controlId" :model-value="block.payload.text" :aria-describedby="describedBy" :invalid="invalid" maxlength="50000" required @update:model-value="simpleField('text', $event)" /></BtgFormField>
    <BtgFormField :label="t('authoring.content.blocks.QUOTE.attribution')" v-slot="{ controlId }"><BtgTextInput :id="controlId" :model-value="block.payload.attribution" maxlength="240" @update:model-value="simpleField('attribution', $event)" /></BtgFormField>
    <BtgFormField :label="t('authoring.content.blocks.QUOTE.sourceUrl')" v-slot="{ controlId }"><BtgTextInput :id="controlId" :model-value="block.payload.sourceUrl" maxlength="2048" @update:model-value="simpleField('sourceUrl', $event)" /></BtgFormField>
  </template>
  <template v-else-if="block.type === 'CALLOUT'">
    <BtgFormField :label="t('authoring.content.blocks.CALLOUT.kind')" v-slot="{ controlId }"><BtgSelect :id="controlId" :model-value="block.payload.kind" @update:model-value="calloutKind"><option value="INFO">{{ t('authoring.content.blocks.CALLOUT.kinds.INFO') }}</option><option value="NOTE">{{ t('authoring.content.blocks.CALLOUT.kinds.NOTE') }}</option><option value="WARNING">{{ t('authoring.content.blocks.CALLOUT.kinds.WARNING') }}</option><option value="TIP">{{ t('authoring.content.blocks.CALLOUT.kinds.TIP') }}</option></BtgSelect></BtgFormField>
    <BtgFormField :label="t('authoring.content.blocks.CALLOUT.title')" v-slot="{ controlId }"><BtgTextInput :id="controlId" :model-value="block.payload.title" maxlength="240" @update:model-value="simpleField('title', $event)" /></BtgFormField>
    <AuthoringRichTextEditor :content="block.payload.content" :label="t('authoring.content.blocks.CALLOUT.editorLabel', { position })" @update:content="emit('update', { ...block, payload: { ...block.payload, content: $event } })" />
  </template>
  <template v-else-if="block.type === 'IMAGE'">
    <AuthoringAssetAttachment :draft-id="draftId" :lesson-id="lessonId" :block-key="block.key" type="IMAGE" :label="t('authoring.content.image.label')" :copy="imageAttachmentCopy" :current-asset-key="block.payload.asset.assetKey" @attached="attach('asset', $event.assetKey)" @unavailable="emit('unavailable')">
      <div class="authoring-image-editor__fields">
        <BtgFormField :label="t('authoring.content.image.alternativeText')" :required="!block.payload.decorative" :error="!block.payload.decorative && !block.payload.altText?.trim() ? t('authoring.content.image.altRequired') : undefined" v-slot="{ controlId, describedBy, invalid }"><BtgTextInput :id="controlId" :model-value="block.payload.altText" :aria-describedby="describedBy" :invalid="invalid" maxlength="1000" :required="!block.payload.decorative" :disabled="block.payload.decorative" @update:model-value="imageField('altText', $event)" /></BtgFormField>
        <AuthoringCheckboxField :model-value="block.payload.decorative" :label="t('authoring.content.image.decorative')" :description="t('authoring.content.image.decorativeHelp')" @update:model-value="emit('update', { ...block, payload: { ...block.payload, decorative: $event, altText: $event ? '' : block.payload.altText } })" />
        <BtgFormField :label="t('authoring.content.image.caption')" :description="t('authoring.content.image.captionHelp')" v-slot="{ controlId, describedBy }"><BtgTextInput :id="controlId" :model-value="block.payload.caption" :aria-describedby="describedBy" maxlength="4000" @update:model-value="imageField('caption', $event)" /></BtgFormField>
      </div>
    </AuthoringAssetAttachment>
  </template>
  <template v-else-if="block.type === 'VIDEO'">
    <AuthoringAssetAttachment :draft-id="draftId" :lesson-id="lessonId" :block-key="block.key" type="VIDEO" :label="t('authoring.content.media.video.asset')" :copy="mediaAttachmentCopy(t('authoring.content.media.video.asset'), t('authoring.content.media.assetTypes.video'))" :current-asset-key="block.payload.asset.assetKey" @attached="attach('asset', $event.assetKey)" @unavailable="emit('unavailable')" />
    <AuthoringAssetAttachment :draft-id="draftId" :lesson-id="lessonId" :block-key="block.key" type="DOWNLOAD" :label="t('authoring.content.media.video.captions')" :copy="mediaAttachmentCopy(t('authoring.content.media.video.captions'), t('authoring.content.media.assetTypes.captions'))" :current-asset-key="block.payload.captionsAsset.assetKey" @attached="attach('captionsAsset', $event.assetKey)" @unavailable="emit('unavailable')" />
    <BtgFormField :label="t('authoring.content.media.video.title')" :error="!block.payload.title.trim() ? t('authoring.content.media.video.titleRequired') : undefined" v-slot="{ controlId, describedBy, invalid }"><BtgTextInput :id="controlId" :model-value="block.payload.title" :aria-describedby="describedBy" :invalid="invalid" maxlength="240" required @update:model-value="mediaField('title', $event)" /></BtgFormField>
    <BtgFormField :label="t('authoring.content.media.video.transcript')" :error="!block.payload.transcript?.trim() && !block.payload.transcriptAsset ? t('authoring.content.media.video.transcriptRequired') : undefined" v-slot="{ controlId, describedBy, invalid }"><BtgTextarea :id="controlId" :model-value="block.payload.transcript" :aria-describedby="describedBy" :invalid="invalid" maxlength="50000" @update:model-value="mediaField('transcript', $event)" /></BtgFormField>
  </template>
  <template v-else-if="block.type === 'AUDIO'">
    <AuthoringAssetAttachment :draft-id="draftId" :lesson-id="lessonId" :block-key="block.key" type="AUDIO" :label="t('authoring.content.media.audio.asset')" :copy="mediaAttachmentCopy(t('authoring.content.media.audio.asset'), t('authoring.content.media.assetTypes.audio'))" :current-asset-key="block.payload.asset.assetKey" @attached="attach('asset', $event.assetKey)" @unavailable="emit('unavailable')" />
    <BtgFormField :label="t('authoring.content.media.audio.title')" :error="!block.payload.title.trim() ? t('authoring.content.media.audio.titleRequired') : undefined" v-slot="{ controlId, describedBy, invalid }"><BtgTextInput :id="controlId" :model-value="block.payload.title" :aria-describedby="describedBy" :invalid="invalid" maxlength="240" required @update:model-value="mediaField('title', $event)" /></BtgFormField>
    <BtgFormField :label="t('authoring.content.media.audio.transcript')" :error="!block.payload.transcript?.trim() && !block.payload.transcriptAsset ? t('authoring.content.media.audio.transcriptRequired') : undefined" v-slot="{ controlId, describedBy, invalid }"><BtgTextarea :id="controlId" :model-value="block.payload.transcript" :aria-describedby="describedBy" :invalid="invalid" maxlength="50000" @update:model-value="mediaField('transcript', $event)" /></BtgFormField>
  </template>
  <template v-else-if="block.type === 'DOWNLOAD'">
    <AuthoringAssetAttachment :draft-id="draftId" :lesson-id="lessonId" :block-key="block.key" type="DOWNLOAD" :label="t('authoring.content.media.download.asset')" :copy="mediaAttachmentCopy(t('authoring.content.media.download.asset'), t('authoring.content.media.assetTypes.download'))" :current-asset-key="block.payload.asset.assetKey" @attached="attach('asset', $event.assetKey)" @unavailable="emit('unavailable')" />
    <BtgFormField :label="t('authoring.content.media.download.label')" :error="!block.payload.label.trim() ? t('authoring.content.media.download.labelRequired') : undefined" v-slot="{ controlId, describedBy, invalid }"><BtgTextInput :id="controlId" :model-value="block.payload.label" :aria-describedby="describedBy" :invalid="invalid" maxlength="240" required @update:model-value="mediaField('label', $event)" /></BtgFormField>
    <BtgFormField :label="t('authoring.content.media.download.description')" v-slot="{ controlId }"><BtgTextarea :id="controlId" :model-value="block.payload.description" maxlength="4000" @update:model-value="mediaField('description', $event)" /></BtgFormField>
  </template>
  <template v-else-if="block.type === 'KNOWLEDGE_CHECK'">
    <AuthoringAssessmentAttachment :draft-id="draftId" :lesson-id="lessonId" :block-key="block.key" :current-assessment-key="block.payload.assessmentKey" :copy="assessmentCopy" @attached="attachAssessment" @unavailable="emit('unavailable')" />
  </template>
  <template v-else-if="block.type === 'PLUGIN_WIDGET'">
    <BtgFormField :label="t('authoring.content.widget.selectorLabel')" :error="!courseWidgets?.length ? t('authoring.content.widget.noneAvailable') : undefined" v-slot="{ controlId, describedBy, invalid }">
      <BtgSelect :id="controlId" :model-value="`${block.payload.pluginId}:${block.payload.pluginVersion}:${block.payload.artifactDigest}:${block.payload.widgetId}`" :aria-describedby="describedBy" :invalid="invalid" @update:model-value="chooseWidgetValue">
        <option v-for="widget in courseWidgets" :key="`${widget.pluginId}:${widget.pluginVersion}:${widget.artifactDigest}:${widget.widgetId}`" :value="`${widget.pluginId}:${widget.pluginVersion}:${widget.artifactDigest}:${widget.widgetId}`">{{ widget.widgetName }} · {{ widget.pluginName }} {{ widget.pluginVersion }}</option>
      </BtgSelect>
    </BtgFormField>
    <WidgetConfigurationForm v-if="selectedWidget()?.configuration?.fields.length" :schema="selectedWidget()!.configuration!" :copy="widgetConfigurationCopy" :model-value="block.payload.configuration" :errors="configurationErrors(selectedWidget()!.configuration, block.payload.configuration)" @update:model-value="widgetConfiguration" />
    <p v-else class="authoring-section__intro">{{ t('authoring.content.widget.noConfiguration') }}</p>
    <p class="authoring-section__intro">{{ t('authoring.content.widget.pinnedDescription') }}</p>
  </template>
  <p v-else-if="block.type === 'DIVIDER'" class="authoring-section__intro">{{ t('authoring.content.blocks.DIVIDER.description') }}</p>
  <template v-else>
    <p>{{ t('authoring.content.blocks.readOnly', { type: t(`authoring.content.blocks.${block.type}.label`) }) }}</p>
  </template>
</template>
