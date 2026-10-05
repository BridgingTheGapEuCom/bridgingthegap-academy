<template>
  <section class="authoring-prerequisites" :aria-label="t('authoring.prerequisites.regionLabel')">
    <div v-if="state === 'loading'" class="authoring-prerequisites__state" role="status">{{ t('authoring.prerequisites.loading') }}</div>
    <div v-else-if="state === 'unavailable'" class="authoring-prerequisites__state"><p>{{ t('authoring.prerequisites.unavailable') }}</p><BtgButton variant="secondary" @click="loadCandidates">{{ t('authoring.prerequisites.tryAgain') }}</BtgButton></div>

    <form v-else :aria-busy="saving" novalidate @submit.prevent="save">
      <p v-if="formError" class="authoring-prerequisites__error" role="alert">{{ formError }}</p>
      <p v-if="saveMessage" class="authoring-prerequisites__status" role="status">{{ saveMessage }}</p>
      <div v-if="conflict" class="authoring-prerequisites__conflict" role="status"><p>{{ t('authoring.prerequisites.changedElsewhere') }}</p><BtgButton variant="secondary" :disabled="reloading" @click="reloadLatest">{{ reloading ? t('authoring.prerequisites.reloading') : t('authoring.prerequisites.reloadLatest') }}</BtgButton></div>

      <BtgSplitWorkspace variant="balanced" :primary-label="t('authoring.prerequisites.available')" :secondary-label="t('authoring.prerequisites.selected')">
        <template #primary><BtgPanel as="section" class="authoring-prerequisites__pane" padding="default"><header class="authoring-prerequisites__pane-header"><h3 id="authoring-prerequisites-available-title">{{ t('authoring.prerequisites.available') }}</h3><p>{{ t('authoring.prerequisites.availableDescription') }}</p></header><BtgSearchField id="authoring-prerequisites-search" v-model="searchQuery" :label="t('authoring.prerequisites.searchLabel')" :placeholder="t('authoring.prerequisites.searchPlaceholder')" :disabled="saving || reloading" /><p v-if="!availableCandidates.length" class="authoring-prerequisites__empty">{{ t('authoring.prerequisites.noAvailable') }}</p><p v-else-if="!filteredGroups.length" class="authoring-prerequisites__empty">{{ t('authoring.prerequisites.noMatches') }}</p><div v-else class="authoring-prerequisites__candidate-groups" :aria-label="t('authoring.prerequisites.byModule')"><BtgDisclosure v-for="group in filteredGroups" :key="group.moduleId" :model-value="moduleExpanded(group.moduleId)" class="authoring-prerequisites__candidate-group" :label="t('authoring.prerequisites.module', { number: moduleNumber(group.modulePosition) })" @update:model-value="setModuleExpanded(group.moduleId, $event)"><template #label><span class="authoring-prerequisites__module-identity"><BtgIcon name="module" decorative /><b>{{ moduleNumber(group.modulePosition) }}</b><span>{{ group.moduleTitle }}</span></span><small>{{ t('common.counts.lesson', group.lessons.length) }}</small></template><ul><li v-for="candidate in group.lessons" :key="candidate.stable_key" :data-selected="selectedKeys.includes(candidate.stable_key) || undefined"><BtgCheckbox :id="`authoring-prerequisite-candidate-${candidate.stable_key}`" :model-value="selectedKeys.includes(candidate.stable_key)" :disabled="saving || reloading" :label="`${lessonNumber(group.modulePosition, candidate.position)} ${candidate.title}`" :aria-label="t('authoring.prerequisites.selectCandidate', { title: candidate.title })" @update:model-value="toggleCandidate(candidate.stable_key, $event)" /></li></ul></BtgDisclosure></div></BtgPanel></template>
        <template #secondary><BtgPanel as="section" class="authoring-prerequisites__pane" padding="default"><header class="authoring-prerequisites__pane-header"><div><div class="authoring-prerequisites__selected-heading"><h3 id="authoring-prerequisites-selected-title">{{ t('authoring.prerequisites.selected') }}</h3><BtgBadge>{{ t('authoring.prerequisites.selectedCount', { count: selectedKeys.length }) }}</BtgBadge></div><p>{{ t('authoring.prerequisites.selectedDescription') }}</p></div></header><p v-if="!selectedKeys.length" class="authoring-prerequisites__empty">{{ t('authoring.prerequisites.noSelected') }}</p><BtgOrderedList v-else class="authoring-prerequisites__selected" :aria-label="t('authoring.prerequisites.ordered')"><BtgOrderedRow v-for="(key, index) in selectedKeys" :key="key" :index="index" :class="{ 'is-dragging': draggingIndex === index, 'is-drop-before': insertionIndex === index && draggingIndex !== index, 'is-drop-after': insertionIndex === index + 1 && draggingIndex !== index }" :data-prerequisite-index="index"><template #handle><BtgDragHandle :id="`authoring-prerequisite-reorder-${index}`" :label="t('authoring.prerequisites.reorder', { position: index + 1, title: candidateTitle(key) })" :aria-pressed="keyboardIndex === index" :disabled="saving || reloading" @pointerdown="startPointerReorder($event, index)" @keydown="keyboardReorder($event, index)" /></template><div class="authoring-prerequisites__selected-summary"><strong>{{ candidateTitle(key) }}</strong><small>{{ candidateModule(key) }}</small></div><template #actions><BtgButton type="button" variant="secondary" leading-icon="delete" :disabled="saving || reloading" :aria-label="t('authoring.prerequisites.remove', { title: candidateTitle(key) })" @click="removePrerequisite(index)">{{ t('authoring.prerequisites.remove', { title: candidateTitle(key) }) }}</BtgButton></template></BtgOrderedRow></BtgOrderedList><p class="sr-only" aria-live="polite">{{ reorderAnnouncement }}</p></BtgPanel></template>
      </BtgSplitWorkspace>
      <AuthoringDirtyActionBar :show="dirty" :busy="saving" :disabled="saving || reloading" :save-disabled="conflict" :message="t('authoring.prerequisites.dirty')" :discard-label="t('authoring.prerequisites.discard')" :save-label="t('authoring.prerequisites.save')" :busy-label="t('authoring.prerequisites.saving')" @discard="discardChanges" />
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { APIProblemError } from '../api/client'
import { preserveFocusAfterRemoval } from '../authoring/focus'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { getAuthoringLesson, getAuthoringStructure, InvalidAuthoringDraftIDError, replaceAuthoringLessonPrerequisites, type AuthoringLessonDetail, type AuthoringLessonSummary } from '../authoring/authoring'
import AuthoringDirtyActionBar from './AuthoringDirtyActionBar.vue'
import BtgButton from './BtgButton.vue'
import BtgSearchField from './BtgSearchField.vue'
import BtgBadge from './BtgBadge.vue'
import BtgCheckbox from './BtgCheckbox.vue'
import BtgDisclosure from './BtgDisclosure.vue'
import BtgDragHandle from './BtgDragHandle.vue'
import BtgIcon from './BtgIcon.vue'
import BtgOrderedList from './BtgOrderedList.vue'
import BtgOrderedRow from './BtgOrderedRow.vue'
import BtgPanel from './BtgPanel.vue'
import BtgSplitWorkspace from './BtgSplitWorkspace.vue'

type CandidateGroup = { moduleId: string; moduleTitle: string; modulePosition: number; lessons: AuthoringLessonSummary[] }
type CandidateDetail = { lesson: AuthoringLessonSummary; moduleTitle: string; modulePosition: number }
const props = defineProps<{ draftId: string; lesson: AuthoringLessonDetail }>()
const { t } = useI18n()
const emit = defineEmits<{ saved: [result: { lesson: import('../api/generated').components['schemas']['AuthoringLessonMutationResponse']; prerequisiteKeys: string[] }]; replaceLesson: [lesson: AuthoringLessonDetail]; unavailable: [] }>()
const state = ref<'loading' | 'ready' | 'unavailable'>('loading')
const candidateGroups = ref<CandidateGroup[]>([])
const originalKeys = ref<string[]>([])
const selectedKeys = ref<string[]>([])
const searchQuery = ref('')
const expandedModules = ref<Set<string>>(new Set())
const saving = ref(false)
const reloading = ref(false)
const conflict = ref(false)
const formError = ref<string>()
const saveMessage = ref<string>()
const draggingIndex = ref<number>()
const insertionIndex = ref<number>()
const keyboardIndex = ref<number>()
const reorderAnnouncement = ref('')
const dirty = computed(() => JSON.stringify(selectedKeys.value) !== JSON.stringify(originalKeys.value))
const candidateDetails = computed(() => new Map<string, CandidateDetail>(candidateGroups.value.flatMap((group) => group.lessons.map((lesson) => [lesson.stable_key, { lesson, moduleTitle: group.moduleTitle, modulePosition: group.modulePosition }]))))
const availableCandidates = computed(() => candidateGroups.value.flatMap((group) => group.lessons).filter((candidate) => candidate.stable_key !== props.lesson.stable_key))
const filteredGroups = computed(() => {
  const query = searchQuery.value.trim().toLocaleLowerCase()
  return candidateGroups.value.map((group) => {
    const moduleMatches = group.moduleTitle.toLocaleLowerCase().includes(query)
    return { ...group, lessons: group.lessons.filter((candidate) => candidate.stable_key !== props.lesson.stable_key && (!query || moduleMatches || `${candidate.title} ${candidate.description}`.toLocaleLowerCase().includes(query))) }
  }).filter((group) => group.lessons.length)
})
let requestVersion = 0
let active = true
let pointerID: number | undefined
let sourceIndex = -1
const captureScope = useAuthoringAsyncScope(() => `${props.draftId}/${props.lesson.id}`)
watch(() => props.lesson.id, () => { originalKeys.value = [...props.lesson.recommended_prerequisite_keys]; selectedKeys.value = [...props.lesson.recommended_prerequisite_keys]; searchQuery.value = ''; conflict.value = false; void loadCandidates() }, { immediate: true })
watch(() => JSON.stringify(props.lesson.recommended_prerequisite_keys), () => { if (JSON.stringify(props.lesson.recommended_prerequisite_keys) === JSON.stringify(originalKeys.value)) return; if (dirty.value) conflict.value = true; else { originalKeys.value = [...props.lesson.recommended_prerequisite_keys]; selectedKeys.value = [...props.lesson.recommended_prerequisite_keys] } })
onBeforeUnmount(() => { active = false; clearPointerReorder() })
function moduleNumber(position: number) { return String(position + 1).padStart(2, '0') }
function lessonNumber(modulePosition: number, lessonPosition: number) { return `${modulePosition + 1}.${lessonPosition + 1}` }
function candidateDetail(key: string) { return candidateDetails.value.get(key) }
function candidateTitle(key: string) { return candidateDetail(key)?.lesson.title ?? t('authoring.prerequisites.lessonUnavailable') }
function candidateModule(key: string) { const detail = candidateDetail(key); return detail ? t('authoring.prerequisites.moduleContext', { number: moduleNumber(detail.modulePosition), title: detail.moduleTitle }) : t('authoring.prerequisites.moduleUnavailable') }
function moduleExpanded(id: string) { return Boolean(searchQuery.value.trim()) || expandedModules.value.has(id) }
function toggleModule(id: string) { const next = new Set(expandedModules.value); next.has(id) ? next.delete(id) : next.add(id); expandedModules.value = next }
function setModuleExpanded(id: string, expanded: boolean) { if (searchQuery.value.trim()) return; const next = new Set(expandedModules.value); expanded ? next.add(id) : next.delete(id); expandedModules.value = next }
function toggleCandidate(key: string, selected: boolean) { if (saving.value || reloading.value || key === props.lesson.stable_key || !candidateDetail(key)) return; if (selected && !selectedKeys.value.includes(key)) selectedKeys.value = [...selectedKeys.value, key]; if (!selected && selectedKeys.value.includes(key)) selectedKeys.value = selectedKeys.value.filter((value) => value !== key) }
function discardChanges() { if (saving.value || reloading.value) return; selectedKeys.value = [...originalKeys.value]; conflict.value = false; formError.value = undefined; saveMessage.value = undefined; keyboardIndex.value = undefined; clearPointerReorder() }
function removePrerequisite(index: number) { if (saving.value || reloading.value) return; const key = selectedKeys.value[index]; if (!key) return; const restoreFocus = preserveFocusAfterRemoval(() => document.querySelector<HTMLElement>(`[data-prerequisite-candidate="${key}"]`)); selectedKeys.value = selectedKeys.value.filter((_, currentIndex) => currentIndex !== index); void restoreFocus() }
function focusDragHandle(index: number) { document.getElementById(`authoring-prerequisite-reorder-${index}`)?.focus() }
function move(from: number, to: number, focus = false) { if (saving.value || reloading.value || from < 0 || to < 0 || from >= selectedKeys.value.length || to >= selectedKeys.value.length || from === to) return false; const next = [...selectedKeys.value]; const [key] = next.splice(from, 1); if (!key) return false; next.splice(to, 0, key); selectedKeys.value = next; reorderAnnouncement.value = t('authoring.prerequisites.moved', { title: candidateTitle(key), position: to + 1, total: next.length }); if (focus) void nextTick(() => focusDragHandle(to)); return true }
function clearPointerReorder() { document.removeEventListener('pointermove', updatePointerReorder); document.removeEventListener('pointerup', finishPointerReorder); document.removeEventListener('pointercancel', cancelPointerReorder); pointerID = undefined; sourceIndex = -1; draggingIndex.value = undefined; insertionIndex.value = undefined }
function startPointerReorder(event: PointerEvent, index: number) { if (saving.value || reloading.value || event.button !== 0) return; event.preventDefault(); keyboardIndex.value = undefined; pointerID = event.pointerId; sourceIndex = index; draggingIndex.value = index; insertionIndex.value = index; document.addEventListener('pointermove', updatePointerReorder); document.addEventListener('pointerup', finishPointerReorder); document.addEventListener('pointercancel', cancelPointerReorder) }
function updatePointerReorder(event: PointerEvent) { if (event.pointerId !== pointerID || draggingIndex.value === undefined) return; const row = document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>('[data-prerequisite-index]'); const target = Number(row?.dataset.prerequisiteIndex); if (!row || !Number.isInteger(target)) return; const bounds = row.getBoundingClientRect(); insertionIndex.value = event.clientY < bounds.top + bounds.height / 2 ? target : target + 1 }
function finishPointerReorder(event: PointerEvent) { if (event.pointerId !== pointerID) return; const from = sourceIndex; const insertion = insertionIndex.value ?? from; clearPointerReorder(); move(from, insertion > from ? insertion - 1 : insertion, true) }
function cancelPointerReorder(event: PointerEvent) { if (event.pointerId !== pointerID) return; const from = sourceIndex; clearPointerReorder(); if (from >= 0) void nextTick(() => focusDragHandle(from)) }
function keyboardReorder(event: KeyboardEvent, index: number) { if (saving.value || reloading.value) return; if (event.key === ' ' || event.key === 'Enter') { event.preventDefault(); keyboardIndex.value = keyboardIndex.value === index ? undefined : index; reorderAnnouncement.value = keyboardIndex.value === index ? t('authoring.prerequisites.reordering', { title: candidateTitle(selectedKeys.value[index] ?? '') }) : t('authoring.prerequisites.finishedReordering'); return }; if (event.key === 'Escape' && keyboardIndex.value === index) { event.preventDefault(); keyboardIndex.value = undefined; reorderAnnouncement.value = t('authoring.prerequisites.finishedReordering'); return }; if (keyboardIndex.value !== index || (event.key !== 'ArrowUp' && event.key !== 'ArrowDown')) return; event.preventDefault(); move(index, index + (event.key === 'ArrowUp' ? -1 : 1), true); keyboardIndex.value = undefined }
function clearError() { formError.value = undefined }
async function loadCandidates() { const isCurrent = captureScope(); const generation = ++requestVersion; state.value = 'loading'; try { const structure = await getAuthoringStructure(props.draftId); if (!isCurrent() || !active || generation !== requestVersion) return; candidateGroups.value = structure.modules.map((module) => ({ moduleId: module.id, moduleTitle: module.title, modulePosition: module.position, lessons: module.lessons })); expandedModules.value = new Set(candidateGroups.value.filter((group) => group.lessons.some((candidate) => candidate.stable_key !== props.lesson.stable_key)).map((group) => group.moduleId)); state.value = 'ready' } catch (error) { if (!isCurrent() || !active || generation !== requestVersion) return; if (error instanceof InvalidAuthoringDraftIDError || (error instanceof APIProblemError && error.status === 404)) { emit('unavailable'); return }; state.value = 'unavailable' } }
async function save() { const isCurrent = captureScope(); if (saving.value || reloading.value || conflict.value || !dirty.value) return; if (selectedKeys.value.includes(props.lesson.stable_key) || new Set(selectedKeys.value).size !== selectedKeys.value.length || selectedKeys.value.some((key) => !candidateDetail(key))) { formError.value = t('authoring.prerequisites.reloadBeforeSaving'); return }; saving.value = true; clearError(); saveMessage.value = undefined; try { const keys = [...selectedKeys.value]; const lesson = await replaceAuthoringLessonPrerequisites(props.draftId, props.lesson.id, { expectedLessonRevision: props.lesson.revision, prerequisiteLessonKeys: keys }); if (!isCurrent()) return; originalKeys.value = keys; emit('saved', { lesson, prerequisiteKeys: keys }); saveMessage.value = t('authoring.prerequisites.saved') } catch (error) { if (!isCurrent()) return; if (error instanceof APIProblemError && error.status === 404) { emit('unavailable'); return }; if (error instanceof APIProblemError && error.status === 409) { conflict.value = true; return }; formError.value = error instanceof APIProblemError && error.status === 400 ? t('authoring.prerequisites.saveInvalid') : t('authoring.prerequisites.saveUnavailable') } finally { if (isCurrent()) saving.value = false } }
async function reloadLatest() { const isCurrent = captureScope(); if (reloading.value || saving.value) return; reloading.value = true; clearError(); saveMessage.value = undefined; try { const latest = await getAuthoringLesson(props.draftId, props.lesson.id); if (!isCurrent()) return; originalKeys.value = [...latest.recommended_prerequisite_keys]; selectedKeys.value = [...latest.recommended_prerequisite_keys]; conflict.value = false; emit('replaceLesson', latest) } catch (error) { if (!isCurrent()) return; if (error instanceof InvalidAuthoringDraftIDError || (error instanceof APIProblemError && error.status === 404)) emit('unavailable'); else formError.value = t('authoring.prerequisites.reloadUnavailable') } finally { if (isCurrent()) reloading.value = false } }
</script>
