<template>
  <section class="authoring-prerequisites" aria-label="Lesson prerequisites">
    <div v-if="state === 'loading'" class="authoring-prerequisites__state" role="status">Loading available lessons…</div>
    <div v-else-if="state === 'unavailable'" class="authoring-prerequisites__state"><p>We couldn’t load available lessons right now.</p><BtgButton variant="secondary" @click="loadCandidates">Try again</BtgButton></div>

    <form v-else :aria-busy="saving" novalidate @submit.prevent="save">
      <p v-if="formError" class="authoring-prerequisites__error" role="alert">{{ formError }}</p>
      <p v-if="saveMessage" class="authoring-prerequisites__status" role="status">{{ saveMessage }}</p>
      <div v-if="conflict" class="authoring-prerequisites__conflict" role="status"><p>This lesson changed elsewhere. Your prerequisite choices are still here, but you need to reload the latest lesson before saving again.</p><BtgButton variant="secondary" :disabled="reloading" @click="reloadLatest">{{ reloading ? 'Reloading…' : 'Reload latest lesson' }}</BtgButton></div>

      <div class="authoring-prerequisites__workspace">
        <section class="authoring-prerequisites__pane authoring-prerequisites__pane--available" aria-labelledby="authoring-prerequisites-available-title">
          <header class="authoring-prerequisites__pane-header"><h3 id="authoring-prerequisites-available-title">Available lessons</h3><p>Browse or search for lessons in this course. The current lesson is not shown.</p></header>
          <BtgSearchField id="authoring-prerequisites-search" v-model="searchQuery" label="Search lessons" placeholder="Search lessons or modules…" :disabled="saving || reloading" />
          <p v-if="!availableCandidates.length" class="authoring-prerequisites__empty">No other lessons are available as prerequisites.</p>
          <p v-else-if="!filteredGroups.length" class="authoring-prerequisites__empty">No lessons match your search.</p>
          <div v-else class="authoring-prerequisites__candidate-groups" aria-label="Available lessons by module">
            <section v-for="group in filteredGroups" :key="group.moduleId" class="authoring-prerequisites__candidate-group">
              <button type="button" class="authoring-prerequisites__module-toggle" :aria-expanded="moduleExpanded(group.moduleId)" :aria-controls="`authoring-prerequisite-module-${group.moduleId}`" @click="toggleModule(group.moduleId)"><span class="authoring-prerequisites__module-identity"><span class="authoring-prerequisites__module-disclosure" aria-hidden="true">{{ moduleExpanded(group.moduleId) ? '⌄' : '›' }}</span><b>{{ moduleNumber(group.modulePosition) }}</b><span>{{ group.moduleTitle }}</span></span><small>{{ group.lessons.length }} {{ group.lessons.length === 1 ? 'lesson' : 'lessons' }}</small></button>
              <ul v-if="moduleExpanded(group.moduleId)" :id="`authoring-prerequisite-module-${group.moduleId}`">
                <li v-for="candidate in group.lessons" :key="candidate.stable_key" :class="{ 'is-selected': selectedKeys.includes(candidate.stable_key) }"><label><input :data-prerequisite-candidate="candidate.stable_key" type="checkbox" :checked="selectedKeys.includes(candidate.stable_key)" :disabled="saving || reloading" :aria-label="`Select ${candidate.title} as a prerequisite`" @change="toggleCandidate(candidate.stable_key, ($event.target as HTMLInputElement).checked)" /><span><small>{{ lessonNumber(group.modulePosition, candidate.position) }}</small>{{ candidate.title }}</span></label></li>
              </ul>
            </section>
          </div>
        </section>

        <section class="authoring-prerequisites__pane authoring-prerequisites__pane--selected" aria-labelledby="authoring-prerequisites-selected-title">
          <header class="authoring-prerequisites__pane-header"><div><div class="authoring-prerequisites__selected-heading"><h3 id="authoring-prerequisites-selected-title">Selected prerequisites</h3><span class="authoring-prerequisites__selected-count">{{ selectedKeys.length }} selected</span></div><p>These recommendations help learners prepare before this lesson. They do not restrict access.</p></div></header>
          <p v-if="!selectedKeys.length" class="authoring-prerequisites__empty">No prerequisites selected. Learners can start this lesson without completing another lesson first.</p>
          <ol v-else class="authoring-prerequisites__selected" aria-label="Ordered recommended prerequisites">
            <li v-for="(key, index) in selectedKeys" :key="key" :class="{ 'is-dragging': draggingIndex === index, 'is-drop-before': insertionIndex === index && draggingIndex !== index, 'is-drop-after': insertionIndex === index + 1 && draggingIndex !== index }" :data-prerequisite-index="index">
              <button :id="`authoring-prerequisite-reorder-${index}`" type="button" class="authoring-prerequisites__drag-handle" :aria-label="`Reorder prerequisite ${index + 1}, ${candidateTitle(key)}`" :aria-pressed="keyboardIndex === index" :disabled="saving || reloading" @pointerdown="startPointerReorder($event, index)" @keydown="keyboardReorder($event, index)"><svg aria-hidden="true" viewBox="0 0 24 24"><path d="M9 5h.01M15 5h.01M9 12h.01M15 12h.01M9 19h.01M15 19h.01" /></svg></button>
              <span class="authoring-prerequisites__number" aria-hidden="true">{{ String(index + 1).padStart(2, '0') }}</span>
              <div class="authoring-prerequisites__selected-summary"><strong>{{ candidateTitle(key) }}</strong><small>{{ candidateModule(key) }}</small></div>
              <div class="authoring-prerequisites__selected-actions"><BtgButton type="button" variant="secondary" :disabled="saving || reloading" :aria-label="`Remove ${candidateTitle(key)}`" @click="removePrerequisite(index)">Remove</BtgButton></div>
            </li>
          </ol>
          <p class="sr-only" aria-live="polite">{{ reorderAnnouncement }}</p>
        </section>
      </div>
      <AuthoringDirtyActionBar :show="dirty" :busy="saving" :disabled="saving || reloading" :save-disabled="conflict" @discard="discardChanges" />
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { APIProblemError } from '../api/client'
import { preserveFocusAfterRemoval } from '../authoring/focus'
import { useAuthoringAsyncScope } from '../authoring/asyncScope'
import { getAuthoringLesson, getAuthoringStructure, InvalidAuthoringDraftIDError, replaceAuthoringLessonPrerequisites, type AuthoringLessonDetail, type AuthoringLessonSummary } from '../authoring/authoring'
import AuthoringDirtyActionBar from './AuthoringDirtyActionBar.vue'
import BtgButton from './BtgButton.vue'
import BtgSearchField from './BtgSearchField.vue'

type CandidateGroup = { moduleId: string; moduleTitle: string; modulePosition: number; lessons: AuthoringLessonSummary[] }
type CandidateDetail = { lesson: AuthoringLessonSummary; moduleTitle: string; modulePosition: number }
const props = defineProps<{ draftId: string; lesson: AuthoringLessonDetail }>()
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
function candidateTitle(key: string) { return candidateDetail(key)?.lesson.title ?? 'Lesson unavailable' }
function candidateModule(key: string) { const detail = candidateDetail(key); return detail ? `Module ${moduleNumber(detail.modulePosition)} · ${detail.moduleTitle}` : 'Module unavailable' }
function moduleExpanded(id: string) { return Boolean(searchQuery.value.trim()) || expandedModules.value.has(id) }
function toggleModule(id: string) { const next = new Set(expandedModules.value); next.has(id) ? next.delete(id) : next.add(id); expandedModules.value = next }
function toggleCandidate(key: string, selected: boolean) { if (saving.value || reloading.value || key === props.lesson.stable_key || !candidateDetail(key)) return; if (selected && !selectedKeys.value.includes(key)) selectedKeys.value = [...selectedKeys.value, key]; if (!selected && selectedKeys.value.includes(key)) selectedKeys.value = selectedKeys.value.filter((value) => value !== key) }
function discardChanges() { if (saving.value || reloading.value) return; selectedKeys.value = [...originalKeys.value]; conflict.value = false; formError.value = undefined; saveMessage.value = undefined; keyboardIndex.value = undefined; clearPointerReorder() }
function removePrerequisite(index: number) { if (saving.value || reloading.value) return; const key = selectedKeys.value[index]; if (!key) return; const restoreFocus = preserveFocusAfterRemoval(() => document.querySelector<HTMLElement>(`[data-prerequisite-candidate="${key}"]`)); selectedKeys.value = selectedKeys.value.filter((_, currentIndex) => currentIndex !== index); void restoreFocus() }
function focusDragHandle(index: number) { document.getElementById(`authoring-prerequisite-reorder-${index}`)?.focus() }
function move(from: number, to: number, focus = false) { if (saving.value || reloading.value || from < 0 || to < 0 || from >= selectedKeys.value.length || to >= selectedKeys.value.length || from === to) return false; const next = [...selectedKeys.value]; const [key] = next.splice(from, 1); if (!key) return false; next.splice(to, 0, key); selectedKeys.value = next; reorderAnnouncement.value = `${candidateTitle(key)} moved to position ${to + 1} of ${next.length}.`; if (focus) void nextTick(() => focusDragHandle(to)); return true }
function clearPointerReorder() { document.removeEventListener('pointermove', updatePointerReorder); document.removeEventListener('pointerup', finishPointerReorder); document.removeEventListener('pointercancel', cancelPointerReorder); pointerID = undefined; sourceIndex = -1; draggingIndex.value = undefined; insertionIndex.value = undefined }
function startPointerReorder(event: PointerEvent, index: number) { if (saving.value || reloading.value || event.button !== 0) return; event.preventDefault(); keyboardIndex.value = undefined; pointerID = event.pointerId; sourceIndex = index; draggingIndex.value = index; insertionIndex.value = index; document.addEventListener('pointermove', updatePointerReorder); document.addEventListener('pointerup', finishPointerReorder); document.addEventListener('pointercancel', cancelPointerReorder) }
function updatePointerReorder(event: PointerEvent) { if (event.pointerId !== pointerID || draggingIndex.value === undefined) return; const row = document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>('[data-prerequisite-index]'); const target = Number(row?.dataset.prerequisiteIndex); if (!row || !Number.isInteger(target)) return; const bounds = row.getBoundingClientRect(); insertionIndex.value = event.clientY < bounds.top + bounds.height / 2 ? target : target + 1 }
function finishPointerReorder(event: PointerEvent) { if (event.pointerId !== pointerID) return; const from = sourceIndex; const insertion = insertionIndex.value ?? from; clearPointerReorder(); move(from, insertion > from ? insertion - 1 : insertion, true) }
function cancelPointerReorder(event: PointerEvent) { if (event.pointerId !== pointerID) return; const from = sourceIndex; clearPointerReorder(); if (from >= 0) void nextTick(() => focusDragHandle(from)) }
function keyboardReorder(event: KeyboardEvent, index: number) { if (saving.value || reloading.value) return; if (event.key === ' ' || event.key === 'Enter') { event.preventDefault(); keyboardIndex.value = keyboardIndex.value === index ? undefined : index; reorderAnnouncement.value = keyboardIndex.value === index ? `Reordering ${candidateTitle(selectedKeys.value[index] ?? '')}. Use the arrow keys to move it, or Escape to finish.` : 'Finished reordering prerequisites.'; return }; if (event.key === 'Escape' && keyboardIndex.value === index) { event.preventDefault(); keyboardIndex.value = undefined; reorderAnnouncement.value = 'Finished reordering prerequisites.'; return }; if (keyboardIndex.value !== index || (event.key !== 'ArrowUp' && event.key !== 'ArrowDown')) return; event.preventDefault(); move(index, index + (event.key === 'ArrowUp' ? -1 : 1), true); keyboardIndex.value = undefined }
function clearError() { formError.value = undefined }
async function loadCandidates() { const isCurrent = captureScope(); const generation = ++requestVersion; state.value = 'loading'; try { const structure = await getAuthoringStructure(props.draftId); if (!isCurrent() || !active || generation !== requestVersion) return; candidateGroups.value = structure.modules.map((module) => ({ moduleId: module.id, moduleTitle: module.title, modulePosition: module.position, lessons: module.lessons })); expandedModules.value = new Set(candidateGroups.value.filter((group) => group.lessons.some((candidate) => candidate.stable_key !== props.lesson.stable_key)).map((group) => group.moduleId)); state.value = 'ready' } catch (error) { if (!isCurrent() || !active || generation !== requestVersion) return; if (error instanceof InvalidAuthoringDraftIDError || (error instanceof APIProblemError && error.status === 404)) { emit('unavailable'); return }; state.value = 'unavailable' } }
async function save() { const isCurrent = captureScope(); if (saving.value || reloading.value || conflict.value || !dirty.value) return; if (selectedKeys.value.includes(props.lesson.stable_key) || new Set(selectedKeys.value).size !== selectedKeys.value.length || selectedKeys.value.some((key) => !candidateDetail(key))) { formError.value = 'Reload the available lessons before saving these recommendations.'; return }; saving.value = true; clearError(); saveMessage.value = undefined; try { const keys = [...selectedKeys.value]; const lesson = await replaceAuthoringLessonPrerequisites(props.draftId, props.lesson.id, { expectedLessonRevision: props.lesson.revision, prerequisiteLessonKeys: keys }); if (!isCurrent()) return; originalKeys.value = keys; emit('saved', { lesson, prerequisiteKeys: keys }); saveMessage.value = 'Prerequisites saved.' } catch (error) { if (!isCurrent()) return; if (error instanceof APIProblemError && error.status === 404) { emit('unavailable'); return }; if (error instanceof APIProblemError && error.status === 409) { conflict.value = true; return }; formError.value = error instanceof APIProblemError && error.status === 400 ? 'We couldn’t save these prerequisites. Check the selected lessons and try again.' : 'We couldn’t save prerequisites right now. Please try again.' } finally { if (isCurrent()) saving.value = false } }
async function reloadLatest() { const isCurrent = captureScope(); if (reloading.value || saving.value) return; reloading.value = true; clearError(); saveMessage.value = undefined; try { const latest = await getAuthoringLesson(props.draftId, props.lesson.id); if (!isCurrent()) return; originalKeys.value = [...latest.recommended_prerequisite_keys]; selectedKeys.value = [...latest.recommended_prerequisite_keys]; conflict.value = false; emit('replaceLesson', latest) } catch (error) { if (!isCurrent()) return; if (error instanceof InvalidAuthoringDraftIDError || (error instanceof APIProblemError && error.status === 404)) emit('unavailable'); else formError.value = 'We couldn’t reload this lesson right now. Please try again.' } finally { if (isCurrent()) reloading.value = false } }
</script>
