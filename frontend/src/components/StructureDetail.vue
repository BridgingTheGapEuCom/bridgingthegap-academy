<template>
  <article v-if="item" class="authoring-structure-detail-panel">
    <template v-if="item.kind === 'module'">
      <header class="authoring-structure-detail-panel__header authoring-structure-detail-panel__module-header">
        <div class="authoring-structure-detail-panel__identity-row">
          <p>{{ t('authoring.structure.inspector.moduleIdentity', { module: number(item.module.position) }) }}</p>
          <BtgButton variant="secondary" size="small" leading-icon="edit" :disabled="busy" @click="$emit('edit-module', item.module, $event)">{{ t('authoring.structure.inspector.editModule') }}</BtgButton>
        </div>
        <h3 class="authoring-structure-detail-panel__title" :title="item.module.title">{{ item.module.title }}</h3>
        <BtgMetadataStrip class="authoring-structure-detail-panel__metadata authoring-structure-detail-panel__module-metadata" :items="moduleMetadata" layout="two-column" />
      </header>

      <AuthoringInspectorSection v-if="item.module.description" :title="t('authoring.structure.inspector.description')" icon="document">
        <p>{{ item.module.description }}</p>
      </AuthoringInspectorSection>

      <AuthoringInspectorSection :title="t('authoring.structure.inspector.otherActions')" icon="settings">
        <div class="authoring-structure-detail-panel__module-actions">
          <BtgButton variant="secondary" leading-icon="arrowUp" :disabled="busy || item.module.position === 0" @click="$emit('move-module', -1, item.module)">{{ t('authoring.structure.inspector.moveUp') }}</BtgButton>
          <BtgButton variant="secondary" leading-icon="arrowDown" :disabled="busy || item.module.position === moduleCount - 1" @click="$emit('move-module', 1, item.module)">{{ t('authoring.structure.inspector.moveDown') }}</BtgButton>
        </div>
      </AuthoringInspectorSection>
      <AuthoringInspectorSection class="authoring-structure-detail-panel__danger-zone" :title="t('authoring.structure.inspector.dangerZone')" icon="delete">
        <p>{{ moduleDeleteDescription }}</p>
        <BtgButton variant="danger-secondary" leading-icon="delete" :disabled="busy || item.module.lessons.length > 0" @click="$emit('delete-module', item.module)">{{ t('authoring.structure.inspector.deleteModule') }}</BtgButton>
      </AuthoringInspectorSection>
    </template>

    <template v-else>
      <header class="authoring-structure-detail-panel__header">
        <div class="authoring-structure-detail-panel__identity-row">
          <p>{{ t('authoring.structure.inspector.lessonIdentity', { module: number(item.module.position), lesson: number(item.lesson.position) }) }}</p>
          <BtgPreviousNextNavigation
            v-if="previous || next"
            :navigation-label="t('authoring.structure.inspector.navigationLabel')"
            :previous-label="t('common.actions.previous')"
            :next-label="t('common.actions.next')"
            :previous-disabled="!previous || busy"
            :next-disabled="!next || busy"
            @previous="previous && $emit('select-lesson', previous)"
            @next="next && $emit('select-lesson', next)"
          />
        </div>
		<h3 class="authoring-structure-detail-panel__lesson-title authoring-structure-detail-panel__title" :title="item.lesson.title">{{ item.lesson.title }}</h3>
        <BtgMetadataStrip class="authoring-structure-detail-panel__metadata" :items="metadata" layout="three-column" />
      </header>

      <AuthoringInspectorSection :title="t('authoring.structure.inspector.description')" icon="document">
        <template #action><BtgButton variant="secondary" size="small" leading-icon="edit" :disabled="busy" @click="$emit('edit-details', item.module, item.lesson)">{{ t('authoring.structure.inspector.editDetails') }}</BtgButton></template>
        <p>{{ item.lesson.description || t('authoring.structure.inspector.noDescription') }}</p>
      </AuthoringInspectorSection>

      <AuthoringInspectorSection :title="t('authoring.structure.inspector.objectives')" icon="objective">
        <template #action><BtgButton variant="secondary" size="small" leading-icon="edit" :disabled="busy" @click="$emit('edit-details', item.module, item.lesson)">{{ t('authoring.structure.inspector.editDetails') }}</BtgButton></template>
        <ul><li v-for="objective in item.lesson.objectives" :key="objective">{{ objective }}</li></ul>
      </AuthoringInspectorSection>

      <AuthoringInspectorSection :title="t('authoring.structure.inspector.content')" icon="content">
        <template #action><BtgButton size="small" leading-icon="edit" :disabled="busy" @click="$emit('edit-content', item.module, item.lesson)">{{ t('authoring.structure.inspector.editContent') }}</BtgButton></template>
        <p v-if="content.kind === 'loading'" role="status">{{ t('authoring.structure.inspector.loadingContent') }}</p>
        <template v-else-if="content.kind === 'unavailable'">
          <p role="alert">{{ t('authoring.structure.inspector.unavailableContent') }}</p>
          <BtgButton variant="secondary" @click="$emit('retry-content', item.lesson)">{{ t('common.actions.tryAgain') }}</BtgButton>
        </template>
        <template v-else-if="content.blocks.length === 0">
          <p><strong>{{ t('authoring.structure.inspector.noContent') }}</strong></p>
          <p>{{ t('authoring.structure.inspector.noContentDescription') }}</p>
        </template>
        <template v-else>
          <p><strong>{{ t('common.counts.block', { count: content.blocks.length }) }}</strong></p>
          <div v-if="blockLabels.length" class="authoring-structure-detail-panel__chips">
            <BtgChip v-for="block in blockLabels" :key="block.type" :icon="blockIcon(block.type)">{{ block.label }}</BtgChip>
          </div>
        </template>
      </AuthoringInspectorSection>

      <AuthoringInspectorSection :title="t('authoring.structure.inspector.otherActions')" icon="settings">
        <BtgButton style="width: 100%" variant="secondary" leading-icon="move" :disabled="busy" @click="$emit('move-lesson', item.lesson, $event)">{{ t('authoring.structure.inspector.moveLesson') }}</BtgButton>
        <div class="authoring-structure-detail-panel__destructive">
          <BtgButton style="width: 100%" variant="danger-secondary" leading-icon="delete" :disabled="busy" @click="$emit('delete-lesson', item.lesson)">{{ t('authoring.structure.inspector.deleteLesson') }}</BtgButton>
        </div>
      </AuthoringInspectorSection>
    </template>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AuthoringLessonSummary, AuthoringModuleSummary } from '../authoring/authoring'
import { useApplicationFormatters } from '../i18n/formatters'
import AuthoringInspectorSection from './AuthoringInspectorSection.vue'
import BtgButton from './BtgButton.vue'
import BtgChip from './BtgChip.vue'
import BtgMetadataStrip from './BtgMetadataStrip.vue'
import BtgPreviousNextNavigation from './BtgPreviousNextNavigation.vue'

type Item =
  | { kind: 'module'; module: AuthoringModuleSummary }
  | { kind: 'lesson'; module: AuthoringModuleSummary; lesson: AuthoringLessonSummary }
type Content =
  | { kind: 'loading' }
  | { kind: 'unavailable' }
  | { kind: 'ready'; blocks: string[]; updatedAt: string }

const props = defineProps<{ item?: Item; busy: boolean; moduleCount: number; content: Content; previous?: Item; next?: Item }>()
defineEmits(['edit-module', 'edit-details', 'edit-content', 'retry-content', 'move-module', 'move-lesson', 'delete-module', 'delete-lesson', 'select-lesson'])
const { t } = useI18n()
const formatters = useApplicationFormatters()

function number(value: number) { return String(value + 1).padStart(2, '0') }
const metadata = computed(() => {
  if (props.item?.kind !== 'lesson') return []
  const { module, lesson } = props.item
  const updatedAt = props.content.kind === 'ready' && props.content.updatedAt ? formatters.date(props.content.updatedAt, 'dateTime') : t('authoring.structure.inspector.notAvailable')
  return [
    { label: t('authoring.structure.inspector.module'), value: `${number(module.position)} · ${module.title}`, icon: 'module' as const },
    { label: t('authoring.structure.inspector.position'), value: t('authoring.structure.lessonPosition', { current: lesson.position + 1, total: module.lessons.length }) },
    { label: t('authoring.structure.inspector.lastUpdated'), value: updatedAt },
  ]
})
const moduleMetadata = computed(() => {
  if (props.item?.kind !== 'module') return []
  return [
    { label: t('authoring.structure.inspector.position'), value: number(props.item.module.position) },
    { label: t('authoring.structure.inspector.lessonCount'), value: t('common.counts.lesson', { count: props.item.module.lessons.length }) },
  ]
})
const moduleDeleteDescription = computed(() => {
  if (props.item?.kind !== 'module') return ''
  const count = props.item.module.lessons.length
  return count === 0
    ? t('authoring.structure.inspector.deleteModuleDescription')
    : t('authoring.structure.inspector.deleteModuleWithLessonsDescription', { count }, count)
})
const blockLabels = computed(() => {
  if (props.content.kind !== 'ready') return []
  const types = [...new Set(props.content.blocks)]
  const visible = types.slice(0, 4).map((type) => ({ type, label: t(`authoring.structure.inspector.blockType.${type}`) }))
  const hidden = types.length - visible.length
  return hidden > 0 ? [...visible, { type: 'MORE', label: t('authoring.structure.inspector.moreContentTypes', { count: hidden }) }] : visible
})
function blockIcon(type: string): 'image' | 'divider' | 'text' | 'callout' {
  return type === 'IMAGE' ? 'image' : type === 'DIVIDER' ? 'divider' : type === 'TEXT' ? 'text' : 'callout'
}
</script>
