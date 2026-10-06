<script setup lang="ts">
import type { CanonicalBlock } from '../authoring/contentEditor'
import type { AuthoringCourseWidget } from '../authoring/authoring'
import BtgFormField from './BtgFormField.vue'
import WidgetConfigurationForm from './WidgetConfigurationForm.vue'
import { configurationErrors, configurationInitial, type WidgetConfiguration } from '../plugins/configuration'
import AuthoringRichTextEditor from './AuthoringRichTextEditor.vue'
import AuthoringInlineTextEditor from './AuthoringInlineTextEditor.vue'
import AuthoringAssetAttachment from './AuthoringAssetAttachment.vue'
import AuthoringCheckboxField from './AuthoringCheckboxField.vue'
import AuthoringAssessmentAttachment from './AuthoringAssessmentAttachment.vue'
import BtgTextInput from './BtgTextInput.vue'
import { useI18n } from 'vue-i18n'
const props = defineProps<{ block: CanonicalBlock; position: number; draftId: string; lessonId: string; courseWidgets?: AuthoringCourseWidget[] }>()
const emit = defineEmits<{ update: [block: CanonicalBlock]; unavailable: [] }>()
const { t } = useI18n()
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
function imageField(name: 'altText' | 'caption', value: string) {
  if (props.block.type !== 'IMAGE') return
  const payload = { ...props.block.payload, [name]: value }
  if (!value) delete payload[name]
  emit('update', { ...props.block, payload })
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
function chooseWidget(event: Event) {
  if (props.block.type !== 'PLUGIN_WIDGET') return
  const selected = props.courseWidgets?.find((widget) => `${widget.pluginId}:${widget.pluginVersion}:${widget.artifactDigest}:${widget.widgetId}` === (event.target as HTMLSelectElement).value)
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
  <AuthoringRichTextEditor v-if="block.type === 'TEXT'" :content="block.payload.content" :label="`Block ${position} text`" code-blocks @update:content="emit('update', { ...block, payload: { ...block.payload, content: $event } })" />
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
    <AuthoringAssetAttachment :draft-id="draftId" :lesson-id="lessonId" :block-key="block.key" type="IMAGE" :label="t('authoring.content.image.choose')" :current-asset-key="block.payload.asset.assetKey" @attached="attach('asset', $event.assetKey)" @unavailable="emit('unavailable')">
      <div class="authoring-image-editor__fields">
        <BtgFormField :label="t('authoring.content.image.alternativeText')" :required="!block.payload.decorative" :error="!block.payload.decorative && !block.payload.altText?.trim() ? t('authoring.content.image.altRequired') : undefined" v-slot="{ controlId, describedBy, invalid }"><BtgTextInput :id="controlId" :model-value="block.payload.altText" :aria-describedby="describedBy" :invalid="invalid" maxlength="1000" :required="!block.payload.decorative" :disabled="block.payload.decorative" @update:model-value="imageField('altText', $event)" /></BtgFormField>
        <AuthoringCheckboxField :model-value="block.payload.decorative" :label="t('authoring.content.image.decorative')" :description="t('authoring.content.image.decorativeHelp')" @update:model-value="emit('update', { ...block, payload: { ...block.payload, decorative: $event, altText: $event ? '' : block.payload.altText } })" />
        <BtgFormField :label="t('authoring.content.image.caption')" :description="t('authoring.content.image.captionHelp')" v-slot="{ controlId, describedBy }"><BtgTextInput :id="controlId" :model-value="block.payload.caption" :aria-describedby="describedBy" maxlength="4000" @update:model-value="imageField('caption', $event)" /></BtgFormField>
      </div>
    </AuthoringAssetAttachment>
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
  <template v-else-if="block.type === 'KNOWLEDGE_CHECK'">
    <AuthoringAssessmentAttachment :draft-id="draftId" :lesson-id="lessonId" :block-key="block.key" :current-assessment-key="block.payload.assessmentKey" @attached="attachAssessment" @unavailable="emit('unavailable')" />
  </template>
  <template v-else-if="block.type === 'PLUGIN_WIDGET'">
    <BtgFormField label="Course widget" :error="!courseWidgets?.length ? 'No Course widgets are currently available.' : undefined" v-slot="{ controlId, describedBy, invalid }">
      <select :id="controlId" :value="`${block.payload.pluginId}:${block.payload.pluginVersion}:${block.payload.artifactDigest}:${block.payload.widgetId}`" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" @change="chooseWidget">
        <option v-for="widget in courseWidgets" :key="`${widget.pluginId}:${widget.pluginVersion}:${widget.artifactDigest}:${widget.widgetId}`" :value="`${widget.pluginId}:${widget.pluginVersion}:${widget.artifactDigest}:${widget.widgetId}`">{{ widget.widgetName }} · {{ widget.pluginName }} {{ widget.pluginVersion }}</option>
      </select>
    </BtgFormField>
    <WidgetConfigurationForm v-if="selectedWidget()?.configuration?.fields.length" :schema="selectedWidget()!.configuration!" :model-value="block.payload.configuration" :errors="configurationErrors(selectedWidget()!.configuration, block.payload.configuration)" @update:model-value="widgetConfiguration" />
    <p v-else class="authoring-section__intro">This widget has no configurable settings.</p>
    <p class="authoring-section__intro">Widget configuration is data only. This release is pinned when the Course is published.</p>
  </template>
  <p v-else-if="block.type === 'DIVIDER'" class="authoring-section__intro">A semantic divider. No configuration is needed.</p>
  <template v-else>
    <p>This {{ block.type.toLowerCase().replaceAll('_', ' ') }} block is preserved read-only. You can move or remove it.</p>
  </template>
</template>
