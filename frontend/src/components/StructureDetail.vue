<template>
  <article v-if="item" class="authoring-structure-detail-panel">
    <template v-if="item.kind === 'module'">
      <header class="authoring-structure-detail-panel__module-header">
        <p><BtgIcon name="module" decorative />{{ t('authoring.structure.inspector.module') }} · {{ t('common.counts.lesson', { count: item.module.lessons.length }) }}</p>
        <h3>{{ item.module.title }}</h3>
        <p v-if="item.module.description">{{ item.module.description }}</p>
      </header>
      <div class="authoring-structure-detail-panel__actions">
        <BtgButton variant="secondary" leading-icon="edit" :disabled="busy" @click="$emit('edit-module', item.module, $event)">{{ t('authoring.structure.inspector.editModule') }}</BtgButton>
        <BtgButton variant="tertiary" :disabled="busy || item.module.position === 0" @click="$emit('move-module', -1, item.module)">{{ t('authoring.structure.inspector.moveUp') }}</BtgButton>
        <BtgButton variant="tertiary" :disabled="busy || item.module.position === moduleCount - 1" @click="$emit('move-module', 1, item.module)">{{ t('authoring.structure.inspector.moveDown') }}</BtgButton>
      </div>
      <div class="authoring-structure-detail-panel__destructive">
        <BtgButton variant="danger-secondary" leading-icon="delete" :disabled="busy || item.module.lessons.length > 0" @click="$emit('delete-module', item.module)">{{ t('authoring.structure.inspector.deleteModule') }}</BtgButton>
      </div>
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
        <h3>{{ item.lesson.title }}</h3>
        <BtgMetadataStrip :items="metadata" />
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
        <BtgButton variant="secondary" leading-icon="move" :disabled="busy" @click="$emit('move-lesson', item.lesson, $event)">{{ t('authoring.structure.inspector.moveLesson') }}</BtgButton>
        <div class="authoring-structure-detail-panel__destructive">
          <BtgButton variant="danger-secondary" leading-icon="delete" :disabled="busy" @click="$emit('delete-lesson', item.lesson)">{{ t('authoring.structure.inspector.deleteLesson') }}</BtgButton>
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
import BtgIcon from './BtgIcon.vue'
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
