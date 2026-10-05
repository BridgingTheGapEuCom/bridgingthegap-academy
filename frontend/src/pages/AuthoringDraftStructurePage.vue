<template>
	<section
		class="authoring-section authoring-structure"
		aria-labelledby="authoring-structure-title"
	>
		<AuthoringPageTitle
			id="authoring-structure-title"
			:title="t('authoring.structure.title')"
			:description="t('authoring.structure.description')"
			focusable
		/>
		<p v-if="state.kind === 'loading'" role="status">
			{{ t('authoring.structure.loading') }}
		</p>
		<div
			v-else-if="state.kind === 'unavailable'"
			class="authoring-structure__state"
		>
				<p>{{ t('authoring.structure.unavailable') }}</p>
				<BtgButton variant="secondary" @click="load">{{ t('common.actions.tryAgain') }}</BtgButton>
		</div>
		<template v-else>
			<p v-if="message" class="authoring-structure__status" role="status">
				{{ message }}
			</p>
			<p v-if="error" class="authoring-structure__error" role="alert">
				{{ error }}
			</p>
			<div
				v-if="conflict"
				class="authoring-structure__conflict"
				role="status"
			>
				<p>
						{{ t('authoring.structure.changedElsewhere') }}
				</p>
				<BtgButton variant="secondary" @click="reload"
						>{{ t('authoring.structure.reload') }}</BtgButton
				>
			</div>
			<div v-if="!modules.length" class="authoring-structure__empty">
					<h3>{{ t('authoring.structure.empty.title') }}</h3>
					<p>{{ t('authoring.structure.empty.description') }}</p>
					<BtgButton leading-icon="plus" @click="openModule()"
						>{{ t('authoring.structure.toolbar.addModule') }}</BtgButton
				>
			</div>
			<div
				class="authoring-structure-workspace-shell"
				:class="{ 'is-reordering': reordering }"
			>
					<BtgToolbar class="authoring-structure-workspace__toolbar">
						<template #leading><BtgSearchField v-model="query" :label="t('authoring.structure.toolbar.searchLabel')" :placeholder="t('authoring.structure.toolbar.searchPlaceholder')" /></template>
						<template #actions>
							<BtgButton variant="quiet" @click="expandAll"
								>{{ t('authoring.structure.toolbar.expandAll') }}</BtgButton
							><BtgButton variant="quiet" @click="collapseAll"
								>{{ t('authoring.structure.toolbar.collapseAll') }}</BtgButton
							><BtgButton
								:disabled="busy || conflict"
								leading-icon="plus"
								@click="openModule(undefined, $event)"
								>{{ t('authoring.structure.toolbar.addModule') }}</BtgButton
							>
						</template>
					</BtgToolbar>
					<BtgSplitWorkspace class="authoring-structure-workspace" variant="outline-inspector">
						<template #primary><BtgPanel
							as="aside"
							class="authoring-structure-outline"
							:aria-label="t('authoring.structure.outline.label')"
						>
						<header class="authoring-structure-outline__header">
								<h3>{{ t('authoring.structure.outline.label') }}</h3>
								<p>
									{{ t('common.counts.module', { count: modules.length }) }}
							</p>
						</header>
						<p
							v-if="!filtered.length"
							class="authoring-structure-outline__empty"
						>
								{{ t('authoring.structure.empty.noMatches') }}
						</p>
						<ol v-else class="authoring-structure-outline__modules">
							<li v-for="module in filtered" :key="module.id">
								<div
									class="authoring-structure-outline__module"
									:class="{
										selected:
											selection?.kind === 'module' &&
											selection.id === module.id,
									}"
								>
									<BtgDisclosure
										:model-value="expanded.has(module.id) || Boolean(query)"
										:label="expanded.has(module.id) || query ? t('authoring.structure.outline.collapseModule', { title: module.title }) : t('authoring.structure.outline.expandModule', { title: module.title })"
										@update:model-value="toggle(module.id)"
									/>
									<BtgButton
										type="button"
										class="authoring-structure-outline__select"
										variant="tertiary"
										:aria-label="t('authoring.structure.outline.selectModule', { title: module.title })"
										:aria-pressed="
											selection?.kind === 'module' &&
											selection.id === module.id
										"
										@click="selectModule(module)"
									>
										<BtgIcon name="module" /><b>{{
											number(module.position)
										}}</b
										><span>{{ module.title }}</span
										><small
											>{{ t('common.counts.lesson', { count: module.lessons.length }) }}</small
										>
									</BtgButton>
								</div>
								<ol
									v-if="expanded.has(module.id) || query"
									class="authoring-structure-outline__lessons"
								>
										<BtgOrderedRow
											v-for="lesson in lessons(module)"
											:key="lesson.id"
											:index="lesson.position"
											:selected="selection?.kind === 'lesson' && selection.id === lesson.id"
										:data-structure-lesson-id="lesson.id"
										:data-structure-module-id="module.id"
										:class="{
											'is-dragging':
												draggingLessonID === lesson.id,
											'is-drop-before':
												draggingLessonID !==
													lesson.id &&
												dragModuleID === module.id &&
												insertionIndex ===
													module.lessons.findIndex(
														(item) =>
															item.id ===
															lesson.id,
													),
											'is-drop-after':
												draggingLessonID !==
													lesson.id &&
												dragModuleID === module.id &&
												insertionIndex ===
													module.lessons.length &&
												lesson.id ===
													module.lessons.at(-1)?.id,
										}"
										>
											<template #handle><BtgDragHandle
											:id="`authoring-structure-reorder-${lesson.id}`"
											type="button"
											class="authoring-structure-outline__drag"
												:label="t('authoring.structure.outline.reorderLesson', { position: number(lesson.position) })"
											:aria-pressed="
												keyboardReorderLessonID ===
												lesson.id
											"
											:aria-describedby="
												query
													? 'authoring-structure-reorder-filtered'
													: 'authoring-structure-reorder-help'
											"
											:disabled="!canReorder"
											@pointerdown="
												startPointerReorder(
													$event,
													module,
													lesson,
												)
											"
											@keydown="
												keyboardReorder(
													$event,
													module,
													lesson,
												)
											"
											/></template>
											<BtgButton
												type="button"
												class="authoring-structure-outline__lesson-select"
												variant="tertiary"
												:aria-label="t('authoring.structure.outline.selectLesson', { title: lesson.title })"
											:aria-pressed="
												selection?.kind === 'lesson' &&
												selection.id === lesson.id
											"
											@click="
												selectLesson(module, lesson)
											"
										>
												<BtgIcon name="lesson" /><span>{{ lesson.title }}</span>
											</BtgButton>
										</BtgOrderedRow>
									<li
										v-if="!module.lessons.length"
										class="authoring-structure-outline__empty-module"
									>
											{{ t('authoring.structure.empty.noLessons') }}
											<BtgButton variant="tertiary" size="small" leading-icon="plus" @click="openLesson(module)">{{ t('authoring.structure.toolbar.addFirstLesson') }}</BtgButton>
									</li>
								</ol>
								<div
									v-if="expanded.has(module.id) || query"
									class="authoring-structure-outline__add"
								>
									<BtgButton
										variant="quiet"
										@click="openLesson(module)"
										leading-icon="plus">{{ t('authoring.structure.toolbar.addLesson') }}</BtgButton
									>
								</div>
							</li>
						</ol>
						<p
							id="authoring-structure-reorder-help"
							class="sr-only"
						>
							{{ t('authoring.structure.outline.reorderHelp') }}
						</p>
						<p
							v-if="query"
							id="authoring-structure-reorder-filtered"
							class="authoring-structure-outline__reorder-note"
						>
							{{ t('authoring.structure.outline.reorderFiltered') }}
						</p>
						<p class="sr-only" aria-live="polite">
							{{ reorderAnnouncement }}
						</p>
						<BtgButton
							class="authoring-structure-outline__add-module"
							variant="secondary"
							:disabled="busy || conflict"
							leading-icon="plus"
							@click="openModule(undefined, $event)"
							>{{ t('authoring.structure.toolbar.addModule') }}</BtgButton
						>
					</BtgPanel></template>
					<template #secondary><BtgPanel
						as="article"
						class="authoring-structure-detail"
						:aria-label="t('authoring.structure.outline.inspectorLabel')"
					>
						<Detail
							v-if="selection"
							:item="current"
							:busy="busy || conflict"
							:module-count="modules.length"
							:content="contentSummary"
							:previous="lessonNavigation.previous"
							:next="lessonNavigation.next"
							@edit-module="openModule"
							@edit-details="openLessonDetails"
							@edit-content="openLessonContent"
							@retry-content="loadContentSummary"
							@move-module="moveModule"
							@move-lesson="openMove"
							@delete-module="removeModule"
							@delete-lesson="removeLesson"
							@select-lesson="selectNavigationLesson"
						/>
						<div v-else class="authoring-structure-detail__empty">
							<h3>{{ t('authoring.structure.inspector.selectLessonTitle') }}</h3>
							<p>{{ t('authoring.structure.inspector.selectLessonDescription') }}</p>
						</div>
					</BtgPanel></template>
					</BtgSplitWorkspace>
			</div>
		</template>
		<BtgDialog v-model:open="moduleDialogOpen" :show-trigger="false" :title="moduleEdit ? t('authoring.structure.dialog.editModuleTitle') : t('authoring.structure.dialog.addModuleTitle')" :description="moduleEdit ? t('authoring.structure.dialog.editModuleDescription') : t('authoring.structure.dialog.addModuleDescription')">
			<form @submit.prevent="saveModule">
				<h3>{{ moduleEdit ? t('authoring.structure.dialog.editModuleTitle') : t('authoring.structure.dialog.addModuleTitle') }}</h3>
				<p>
					{{
						moduleEdit
							? t('authoring.structure.dialog.editModuleDescription')
							: t('authoring.structure.dialog.addModuleDescription')
					}}
				</p>
				<BtgFormField
					:label="t('authoring.structure.dialog.moduleTitle')"
					required
					:error="moduleErrors.title"
					v-slot="p"
					><BtgTextInput
						:id="p.controlId"
						v-model="moduleForm.title"
						:aria-describedby="p.describedBy"
						:invalid="p.invalid"
						required /></BtgFormField
				><BtgFormField :label="t('authoring.structure.dialog.moduleDescription')" v-slot="p">
					<BtgTextarea
						:id="p.controlId"
						v-model="moduleForm.description"
						:aria-describedby="p.describedBy"
					/>
				</BtgFormField>
				<div class="authoring-structure-dialog__actions">
					<BtgButton
						variant="secondary"
						@click.prevent="moduleDialogOpen = false"
						>{{ t('authoring.structure.dialog.cancel') }}</BtgButton
					><BtgButton type="submit" :disabled="busy">{{
						moduleEdit ? t('authoring.structure.dialog.saveModule') : t('authoring.structure.dialog.createModule')
					}}</BtgButton>
				</div>
			</form>
		</BtgDialog>
		<BtgDialog v-model:open="lessonDialogOpen" :show-trigger="false" :title="lessonEdit ? t('authoring.structure.dialog.editLessonTitle') : t('authoring.structure.dialog.addLessonTitle')" :description="lessonEdit ? t('authoring.structure.dialog.editLessonDescription') : t('authoring.structure.dialog.addLessonDescription')">
			<form @submit.prevent="saveLesson">
				<h3>{{ lessonEdit ? t('authoring.structure.dialog.editLessonTitle') : t('authoring.structure.dialog.addLessonTitle') }}</h3>
				<p>
					{{
						lessonEdit
							? t('authoring.structure.dialog.editLessonDescription')
							: t('authoring.structure.dialog.addLessonDescription')
					}}
				</p>
				<BtgFormField
					:label="t('authoring.structure.dialog.lessonTitle')"
					required
					:error="lessonErrors.title"
					v-slot="p"
					><BtgTextInput
						:id="p.controlId"
						v-model="lessonForm.title"
						:placeholder="t('authoring.structure.dialog.lessonTitlePlaceholder')"
						:aria-describedby="p.describedBy"
						:invalid="p.invalid"
						required /></BtgFormField
				><BtgFormField
					:label="t('authoring.structure.dialog.lessonDescription')"
					required
					:description="t('authoring.structure.dialog.lessonDescriptionHelp')"
					:error="lessonErrors.description"
					v-slot="p"
				>
					<BtgTextarea
						:id="p.controlId"
						v-model="lessonForm.description"
						:aria-describedby="p.describedBy"
						:aria-invalid="p.invalid || undefined"
						required
					/></BtgFormField
				><RepeatableObjectivesEditor
					v-model="lessonForm.objectives"
					:label="t('authoring.structure.dialog.objectives')"
					:description="t('authoring.structure.dialog.objectivesHelp')"
					:error="lessonErrors.objectives"
					:disabled="busy"
				/>
				<div class="authoring-structure-dialog__actions">
					<BtgButton
						variant="secondary"
						@click.prevent="lessonDialogOpen = false"
						>{{ t('authoring.structure.dialog.cancel') }}</BtgButton
					><BtgButton type="submit" :disabled="busy">{{
						lessonEdit ? t('authoring.structure.dialog.saveLesson') : t('authoring.structure.dialog.createLesson')
					}}</BtgButton>
				</div>
			</form>
		</BtgDialog>
		<BtgDialog v-model:open="moveDialogOpen" :show-trigger="false" :title="t('authoring.structure.dialog.moveLessonTitle')">
			<form @submit.prevent="saveMove">
				<h3>{{ t('authoring.structure.dialog.moveLessonTitle') }}</h3>
				<p>
					{{ t('authoring.structure.dialog.moveLessonDescription', { title: moving?.title }) }}
				</p>
				<BtgFormField :label="t('authoring.structure.dialog.module')" v-slot="p"
					><BtgSelect
						:id="p.controlId"
						v-model="targetModule"
						:aria-describedby="p.describedBy"
					>
						<option
							v-for="module in modules"
							:key="module.id"
							:value="module.id"
						>
							{{ module.title }}
						</option>
					</BtgSelect></BtgFormField
				><BtgFormField :label="t('authoring.structure.dialog.position')" v-slot="p"
					><BtgSelect
						:id="p.controlId"
						:model-value="String(targetPosition)"
						@update:model-value="targetPosition = Number($event)"
						:aria-describedby="p.describedBy"
					>
						<option
							v-for="(label, index) in positions"
							:key="index"
							:value="index"
						>
							{{ label }}
						</option>
					</BtgSelect></BtgFormField
				>
				<div class="authoring-structure-dialog__actions">
					<BtgButton
						variant="secondary"
						@click.prevent="moveDialogOpen = false"
						>{{ t('authoring.structure.dialog.cancel') }}</BtgButton
					><BtgButton type="submit" :disabled="busy"
						>{{ t('authoring.structure.inspector.moveLesson') }}</BtgButton
					>
				</div>
			</form>
		</BtgDialog>
	</section>
</template>
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { APIProblemError } from "../api/client";
import { useAuthoringAsyncScope } from "../authoring/asyncScope";
import {
	authoringDraftPath,
	createAuthoringLesson,
	createAuthoringModule,
	deleteAuthoringLesson,
	deleteAuthoringModule,
	getAuthoringDraft,
	getAuthoringLesson,
	getAuthoringStructure,
	reorderAuthoringLessons,
	reorderAuthoringModules,
	updateAuthoringLesson,
	updateAuthoringModule,
	type AuthoringLessonSummary,
	type AuthoringModuleSummary,
	type AuthoringStructure,
} from "../authoring/authoring";
import { useAuthoringDraftContext } from "../authoring/draftContext";
import BtgButton from "../components/BtgButton.vue";
import BtgIcon from "../components/BtgIcon.vue";
import BtgPanel from "../components/BtgPanel.vue";
import BtgSearchField from "../components/BtgSearchField.vue";
import BtgToolbar from "../components/BtgToolbar.vue";
import BtgSplitWorkspace from "../components/BtgSplitWorkspace.vue";
import BtgDisclosure from "../components/BtgDisclosure.vue";
import BtgOrderedRow from "../components/BtgOrderedRow.vue";
import BtgDragHandle from "../components/BtgDragHandle.vue";
import BtgDialog from "../components/BtgDialog.vue";
import BtgFormField from "../components/BtgFormField.vue";
import BtgTextInput from "../components/BtgTextInput.vue";
import BtgTextarea from "../components/BtgTextarea.vue";
import BtgSelect from "../components/BtgSelect.vue";
import RepeatableObjectivesEditor from "../components/RepeatableObjectivesEditor.vue";
import Detail from "../components/StructureDetail.vue";
import AuthoringPageTitle from "../components/AuthoringPageTitle.vue";
const { t } = useI18n();
type State =
	| { kind: "loading" }
	| { kind: "ready"; structure: AuthoringStructure }
	| { kind: "unavailable" };
type Selected = { kind: "module"; id: string } | { kind: "lesson"; id: string };
const route = useRoute();
const router = useRouter();
const { draft, replaceDraft, markDraftUnavailable } =
	useAuthoringDraftContext();
const state = ref<State>({ kind: "loading" });
const busy = ref(false);
const reordering = ref(false);
const conflict = ref(false);
const message = ref<string>();
const error = ref<string>();
const query = ref(
	typeof route.query.search === "string" ? route.query.search : "",
);
const expanded = ref(new Set<string>());
const selection = ref<Selected>();
const moduleDialogOpen = ref(false);
const lessonDialogOpen = ref(false);
const moveDialogOpen = ref(false);
const lastFocus = ref<HTMLElement>();
const moduleEdit = ref<AuthoringModuleSummary>();
const lessonEdit = ref<AuthoringLessonSummary>();
const lessonModule = ref<AuthoringModuleSummary>();
const moving = ref<AuthoringLessonSummary>();
const targetModule = ref("");
const targetPosition = ref(0);
const moduleForm = reactive({ title: "", description: "", objectives: [""] });
const lessonForm = reactive({ title: "", description: "", objectives: [""] });
const moduleErrors = reactive<Record<string, string | undefined>>({});
const lessonErrors = reactive<Record<string, string | undefined>>({});
const content = ref<
	| { kind: "loading"; lessonID: string }
	| { kind: "unavailable"; lessonID: string }
	| { kind: "ready"; lessonID: string; blocks: string[]; updatedAt: string }
>({ kind: "ready", lessonID: "", blocks: [], updatedAt: "" });
const reorderAnnouncement = ref("");
const draggingLessonID = ref<string>();
const dragModuleID = ref<string>();
const insertionIndex = ref<number>();
const keyboardReorderLessonID = ref<string>();
let dragPointerID: number | undefined;
let dragSourceIndex = -1;
let alive = true;
let version = 0;
let contentVersion = 0;
const scope = useAuthoringAsyncScope(() => draft.value.id);
const modules = computed(() =>
	state.value.kind === "ready" ? state.value.structure.modules : [],
);
const canReorder = computed(
	() => !busy.value && !conflict.value && !query.value.trim(),
);
const filtered = computed(() => {
	const q = query.value.trim().toLowerCase();
	return !q
		? modules.value
		: modules.value.filter(
				(m) =>
					`${m.title} ${m.description}`.toLowerCase().includes(q) ||
					m.lessons.some((l) =>
						`${l.title} ${l.description}`.toLowerCase().includes(q),
					),
			);
});
const current = computed(() => {
	for (const module of modules.value) {
		if (
			selection.value?.kind === "module" &&
			selection.value.id === module.id
		)
			return { kind: "module" as const, module };
		const lesson = module.lessons.find((l) => l.id === selection.value?.id);
		if (lesson) return { kind: "lesson" as const, module, lesson };
	}
	return undefined;
});
const lessonNavigation = computed(() => {
	const selected = current.value;
	if (selected?.kind !== "lesson") return {};
	const lessons = modules.value.flatMap((module) =>
		module.lessons.map((lesson) => ({
			kind: "lesson" as const,
			module,
			lesson,
		})),
	);
	const index = lessons.findIndex(
		(item) => item.lesson.id === selected.lesson.id,
	);
	return { previous: lessons[index - 1], next: lessons[index + 1] };
});
const contentSummary = computed(() =>
	current.value?.kind === "lesson" &&
	content.value.lessonID === current.value.lesson.id
		? content.value.kind === "ready"
			? {
					kind: "ready" as const,
					blocks: content.value.blocks,
					updatedAt: content.value.updatedAt,
				}
			: { kind: content.value.kind }
		: { kind: "ready" as const, blocks: [], updatedAt: "" },
);
const positions = computed(() => {
	const m = modules.value.find((x) => x.id === targetModule.value);
	const rest = m?.lessons.filter((x) => x.id !== moving.value?.id) ?? [];
	return [t('authoring.structure.dialog.atBeginning'), ...rest.map((x) => t('authoring.structure.dialog.afterLesson', { title: x.title }))];
});
watch(
	() => draft.value.id,
	() => {
		void load();
	},
	{ immediate: true },
);
watch(query, () => {
	if (query.value) {
		clearPointerReorder();
		keyboardReorderLessonID.value = undefined;
		expanded.value = new Set(filtered.value.map((m) => m.id));
	}
});
watch(targetModule, () => {
	targetPosition.value = Math.min(
		targetPosition.value,
		positions.value.length - 1,
	);
});
watch([moduleDialogOpen, lessonDialogOpen, moveDialogOpen], (open, previous) => {
	if (previous.some(Boolean) && !open.some(Boolean)) restoreFocus();
});
onBeforeUnmount(() => {
	alive = false;
	clearPointerReorder();
});
function number(n: number) {
	return String(n + 1).padStart(2, "0");
}
function lessons(m: AuthoringModuleSummary) {
	const q = query.value.trim().toLowerCase();
	return q
		? m.lessons.filter((l) =>
				`${l.title} ${l.description}`.toLowerCase().includes(q),
			)
		: m.lessons;
}
function toggle(id: string) {
	const x = new Set(expanded.value);
	x.has(id) ? x.delete(id) : x.add(id);
	expanded.value = x;
}
function expandAll() {
	expanded.value = new Set(modules.value.map((m) => m.id));
}
function collapseAll() {
	expanded.value = new Set();
}
function clearSelection() {
	contentVersion += 1;
	selection.value = undefined;
}
function selectModule(m: AuthoringModuleSummary) {
	contentVersion += 1;
	selection.value = { kind: "module", id: m.id };
	expanded.value = new Set(expanded.value).add(m.id);
}
function selectLesson(m: AuthoringModuleSummary, l: AuthoringLessonSummary) {
	selection.value = { kind: "lesson", id: l.id };
	expanded.value = new Set(expanded.value).add(m.id);
	void loadContentSummary(l);
}
function selectNavigationLesson(item: {
	kind: "lesson";
	module: AuthoringModuleSummary;
	lesson: AuthoringLessonSummary;
}) {
	selectLesson(item.module, item.lesson);
}
function focusReorderHandle(lessonID: string) {
	document
		.getElementById(`authoring-structure-reorder-${lessonID}`)
		?.focus({ preventScroll: true });
}
function clearPointerReorder() {
	document.removeEventListener("pointermove", updatePointerReorder);
	document.removeEventListener("pointerup", finishPointerReorder);
	document.removeEventListener("pointercancel", cancelPointerReorder);
	dragPointerID = undefined;
	dragSourceIndex = -1;
	draggingLessonID.value = undefined;
	dragModuleID.value = undefined;
	insertionIndex.value = undefined;
}
function startPointerReorder(
	event: PointerEvent,
	module: AuthoringModuleSummary,
	lesson: AuthoringLessonSummary,
) {
	if (!canReorder.value || event.button !== 0) return;
	event.preventDefault();
	keyboardReorderLessonID.value = undefined;
	dragPointerID = event.pointerId;
	dragSourceIndex = module.lessons.findIndex((item) => item.id === lesson.id);
	draggingLessonID.value = lesson.id;
	dragModuleID.value = module.id;
	insertionIndex.value = dragSourceIndex;
	document.addEventListener("pointermove", updatePointerReorder);
	document.addEventListener("pointerup", finishPointerReorder);
	document.addEventListener("pointercancel", cancelPointerReorder);
}
function updatePointerReorder(event: PointerEvent) {
	if (
		event.pointerId !== dragPointerID ||
		!dragModuleID.value ||
		!draggingLessonID.value
	)
		return;
	const row = document
		.elementFromPoint(event.clientX, event.clientY)
		?.closest<HTMLElement>("[data-structure-lesson-id]");
	if (!row || row.dataset.structureModuleId !== dragModuleID.value) return;
	const targetLessonID = row.dataset.structureLessonId;
	const module = modules.value.find((item) => item.id === dragModuleID.value);
	const target =
		module?.lessons.findIndex((item) => item.id === targetLessonID) ?? -1;
	if (target < 0) return;
	const bounds = row.getBoundingClientRect();
	insertionIndex.value =
		event.clientY < bounds.top + bounds.height / 2 ? target : target + 1;
}
function finishPointerReorder(event: PointerEvent) {
	if (event.pointerId !== dragPointerID) return;
	const lessonID = draggingLessonID.value;
	const moduleID = dragModuleID.value;
	const source = dragSourceIndex;
	const insertion = insertionIndex.value ?? source;
	clearPointerReorder();
	if (!lessonID || !moduleID || source < 0) return;
	const module = modules.value.find((item) => item.id === moduleID);
	const lesson = module?.lessons.find((item) => item.id === lessonID);
	const destination = insertion > source ? insertion - 1 : insertion;
	if (!module || !lesson || destination === source) return;
	void moveLesson(lesson, moduleID, destination);
}
function cancelPointerReorder(event: PointerEvent) {
	if (event.pointerId !== dragPointerID) return;
	const lessonID = draggingLessonID.value;
	clearPointerReorder();
	if (lessonID) void nextTick(() => focusReorderHandle(lessonID));
}
function keyboardReorder(
	event: KeyboardEvent,
	module: AuthoringModuleSummary,
	lesson: AuthoringLessonSummary,
) {
	if (!canReorder.value) return;
	if (event.key === " " || event.key === "Enter") {
		event.preventDefault();
		keyboardReorderLessonID.value =
			keyboardReorderLessonID.value === lesson.id ? undefined : lesson.id;
		reorderAnnouncement.value =
			keyboardReorderLessonID.value === lesson.id
				? t('authoring.structure.outline.reordering', { title: lesson.title })
				: t('authoring.structure.outline.finishedReordering', { title: lesson.title });
		return;
	}
	if (event.key === "Escape" && keyboardReorderLessonID.value === lesson.id) {
		event.preventDefault();
		keyboardReorderLessonID.value = undefined;
		reorderAnnouncement.value = t('authoring.structure.outline.finishedReordering', { title: lesson.title });
		return;
	}
	if (
		keyboardReorderLessonID.value !== lesson.id ||
		(event.key !== "ArrowUp" && event.key !== "ArrowDown")
	)
		return;
	event.preventDefault();
	const source = module.lessons.findIndex((item) => item.id === lesson.id);
	const destination = source + (event.key === "ArrowUp" ? -1 : 1);
	keyboardReorderLessonID.value = undefined;
	if (destination < 0 || destination >= module.lessons.length) return;
	void moveLesson(lesson, module.id, destination, true);
}
async function loadContentSummary(lesson?: AuthoringLessonSummary) {
	const selected =
		lesson ??
		(current.value?.kind === "lesson" ? current.value.lesson : undefined);
	if (!selected) return;
	const currentScope = scope();
	const request = ++contentVersion;
	// content.value = { kind: "loading", lessonID: selected.id };
	try {
		const detail = await getAuthoringLesson(draft.value.id, selected.id);
		if (
			!currentScope() ||
			!alive ||
			request !== contentVersion ||
			selection.value?.kind !== "lesson" ||
			selection.value.id !== selected.id
		)
			return;
		content.value = {
			kind: "ready",
			lessonID: selected.id,
			blocks: detail.content.blocks.map((block) => block.type),
			updatedAt: detail.updated_at,
		};
	} catch {
		if (
			!currentScope() ||
			!alive ||
			request !== contentVersion ||
			selection.value?.kind !== "lesson" ||
			selection.value.id !== selected.id
		)
			return;
		content.value = { kind: "unavailable", lessonID: selected.id };
	}
}
function openLessonDetails(
	_module: AuthoringModuleSummary,
	lesson: AuthoringLessonSummary,
) {
	void openLessonEditor(lesson, "authoring-draft-lesson");
}
function openLessonContent(
	_module: AuthoringModuleSummary,
	lesson: AuthoringLessonSummary,
) {
	void openLessonEditor(lesson, "authoring-draft-lesson-content");
}
async function openLessonEditor(
	lesson: AuthoringLessonSummary,
	name: "authoring-draft-lesson" | "authoring-draft-lesson-content",
) {
	await router.push({
		name,
		params: { draftId: draft.value.id, lessonId: lesson.id },
		query: {
			from: "structure",
			lesson: lesson.id,
			search: query.value || undefined,
		},
	});
}
function dialog(e?: Event) {
	lastFocus.value =
		e?.currentTarget instanceof HTMLElement
			? e.currentTarget
			: document.activeElement instanceof HTMLElement
				? document.activeElement
				: undefined;
}
function restoreFocus() {
	nextTick(() => lastFocus.value?.focus());
}
function clear(x: Record<string, string | undefined>) {
	Object.keys(x).forEach((k) => delete x[k]);
}
function openModule(m?: AuthoringModuleSummary, e?: Event) {
	moduleEdit.value = m;
	moduleForm.title = m?.title ?? "";
	moduleForm.description = m?.description ?? "";
	clear(moduleErrors);
	dialog(e);
	moduleDialogOpen.value = true;
}
function openLesson(
	m?: AuthoringModuleSummary,
	l?: AuthoringLessonSummary,
	e?: Event,
) {
	lessonModule.value = m ?? current.value?.module;
	lessonEdit.value = l;
	lessonForm.title = l?.title ?? "";
	lessonForm.description = l?.description ?? "";
	lessonForm.objectives = l ? [...l.objectives] : [""];
	clear(lessonErrors);
	dialog(e);
	lessonDialogOpen.value = true;
}
function openMove(l?: AuthoringLessonSummary, e?: Event) {
	const lesson = l ?? current.value?.lesson;
	if (!lesson) return;
	moving.value = lesson;
	const module = modules.value.find((m) =>
		m.lessons.some((x) => x.id === lesson.id),
	);
	targetModule.value = module?.id ?? "";
	targetPosition.value =
		module?.lessons.findIndex((x) => x.id === lesson.id) ?? 0;
	dialog(e);
	moveDialogOpen.value = true;
}
async function load(refresh = false) {
	const currentScope = scope();
	const request = ++version;
	if (!refresh) state.value = { kind: "loading" };
	try {
		const structure = await getAuthoringStructure(draft.value.id);
		if (!currentScope() || !alive || request !== version) return;
		state.value = { kind: "ready", structure };
		const returnedLessonID =
			typeof route.query.lesson === "string"
				? route.query.lesson
				: undefined;
		const restored = returnedLessonID
			? modules.value.find((m) =>
					m.lessons.some((l) => l.id === returnedLessonID),
				)
			: undefined;
		if (restored) {
			const lesson = restored.lessons.find(
				(l) => l.id === returnedLessonID,
			)!;
			selectLesson(restored, lesson);
		} else {
			const parent = modules.value.find((m) =>
				m.lessons.some((l) => l.id === selection.value?.id),
			);
			if (parent) expanded.value = new Set(expanded.value).add(parent.id);
		}
	} catch (cause) {
		if (!currentScope() || !alive || request !== version) return;
		if (cause instanceof APIProblemError && cause.status === 404) {
			markDraftUnavailable();
			return;
		}
		state.value = { kind: "unavailable" };
	}
}
async function reload() {
	busy.value = true;
	try {
		const [d, s] = await Promise.all([
			getAuthoringDraft(draft.value.id),
			getAuthoringStructure(draft.value.id),
		]);
		replaceDraft(d);
		state.value = { kind: "ready", structure: s };
		conflict.value = false;
		message.value = t('authoring.structure.latestLoaded');
	} catch (cause) {
		fail(cause);
	} finally {
		busy.value = false;
	}
}
async function saveModule() {
	clear(moduleErrors);
	if (!moduleForm.title.trim()) moduleErrors.title = t('authoring.structure.validation.moduleTitle');
	if (Object.keys(moduleErrors).length) return;
	await mutate(async () => {
		const r = moduleEdit.value
			? await updateAuthoringModule(draft.value.id, moduleEdit.value.id, {
					expectedModuleRevision: moduleEdit.value.revision,
					title: moduleForm.title,
					description: moduleForm.description,
				})
			: await createAuthoringModule(draft.value.id, {
					expectedDraftRevision: draft.value.revision,
					title: moduleForm.title,
					...(moduleForm.description
						? { description: moduleForm.description }
						: {}),
				});
		await commit(
			r.draftRevision,
			moduleEdit.value ? t('authoring.structure.feedback.moduleUpdated') : t('authoring.structure.feedback.moduleCreated'),
		);
		moduleDialogOpen.value = false;
	});
}
async function saveLesson() {
	clear(lessonErrors);
	if (!lessonForm.title.trim()) lessonErrors.title = t('authoring.structure.validation.lessonTitle');
	if (!lessonForm.description.trim())
		lessonErrors.description = t('authoring.structure.validation.lessonDescription');
	const objectives = lessonForm.objectives.map((x) => x.trim());
	if (!objectives.length || objectives.some((x) => !x))
		lessonErrors.objectives = t('authoring.structure.validation.objectives');
	if (Object.keys(lessonErrors).length || !lessonModule.value) return;
	await mutate(async () => {
		const r = lessonEdit.value
			? await updateAuthoringLesson(draft.value.id, lessonEdit.value.id, {
					expectedLessonRevision: lessonEdit.value.revision,
					title: lessonForm.title,
					description: lessonForm.description,
					objectives,
				})
			: await createAuthoringLesson(
					draft.value.id,
					lessonModule.value!.id,
					{
						expectedDraftRevision: draft.value.revision,
						title: lessonForm.title,
						description: lessonForm.description,
						objectives,
					},
				);
		await commit(
			r.draftRevision,
			lessonEdit.value ? t('authoring.structure.feedback.lessonUpdated') : t('authoring.structure.feedback.lessonCreated'),
		);
		lessonDialogOpen.value = false;
	});
}
async function moveModule(delta: -1 | 1, m?: AuthoringModuleSummary) {
	const module = m ?? current.value?.module;
	if (!module) return;
	const ids = modules.value.map((x) => x.id);
	const i = ids.indexOf(module.id);
	if (i + delta < 0 || i + delta >= ids.length) return;
	[ids[i], ids[i + delta]] = [ids[i + delta]!, ids[i]!];
	await mutate(async () => {
		const r = await reorderAuthoringModules(
			draft.value.id,
			draft.value.revision,
			ids,
		);
		await commit(r.draftRevision, t('authoring.structure.feedback.moduleOrderUpdated'));
	});
}
async function saveMove() {
	if (!moving.value) return;
	await moveLesson(moving.value, targetModule.value, targetPosition.value);
	moveDialogOpen.value = false;
}
function applyLessonOrder(
	order: Array<{ moduleId: string; lessonIds: string[] }>,
) {
	if (state.value.kind !== "ready") return;
	const lessons = new Map(
		state.value.structure.modules.flatMap((m) =>
			m.lessons.map((l) => [l.id, l]),
		),
	);
	const lessonIDs = new Map(order.map((m) => [m.moduleId, m.lessonIds]));
	for (const module of state.value.structure.modules) {
		const ids = lessonIDs.get(module.id);
		if (!ids) continue;
		module.lessons.splice(
			0,
			module.lessons.length,
			...ids.map((id, position) =>
				Object.assign(lessons.get(id)!, { position }),
			),
		);
	}
}
async function moveLesson(
	l: AuthoringLessonSummary,
	moduleID: string,
	position: number,
	focus = false,
) {
	const order = modules.value.map((m) => ({
		moduleId: m.id,
		lessonIds: m.lessons.map((x) => x.id),
	}));
	const from = order.find((m) => m.lessonIds.includes(l.id));
	const to = order.find((m) => m.moduleId === moduleID);
	if (!from || !to || busy.value || conflict.value) return;
	const sourceIndex = from.lessonIds.indexOf(l.id);
	from.lessonIds.splice(sourceIndex, 1);
	const destination = Math.max(0, Math.min(position, to.lessonIds.length));
	if (from === to && sourceIndex === destination) return;
	to.lessonIds.splice(destination, 0, l.id);
	const text = t('authoring.structure.movedLesson', { title: l.title, position: destination + 1, total: to.lessonIds.length });
	reordering.value = true;
	try {
		await mutate(async () => {
			const r = await reorderAuthoringLessons(
				draft.value.id,
				draft.value.revision,
				order,
			);
			replaceDraft({ ...draft.value, revision: r.draftRevision });
			applyLessonOrder(order);
			reorderAnnouncement.value = text;
			if (focus) await nextTick(() => focusReorderHandle(l.id));
		}, true);
	} finally {
		reordering.value = false;
	}
}
async function removeModule(m?: AuthoringModuleSummary) {
	const module = m ?? current.value?.module;
	if (!module) return;
	if (module.lessons.length) {
		error.value = t('authoring.structure.feedback.moduleHasLessons');
		return;
	}
	await mutate(async () => {
		const r = await deleteAuthoringModule(
			draft.value.id,
			module.id,
			draft.value.revision,
			module.revision,
		);
		selection.value = undefined;
		await commit(r.draftRevision, t('authoring.structure.feedback.emptyModuleDeleted'));
	});
}
async function removeLesson(l?: AuthoringLessonSummary) {
	const lesson = l ?? current.value?.lesson;
	if (!lesson) return;
	await mutate(async () => {
		const r = await deleteAuthoringLesson(
			draft.value.id,
			lesson.id,
			draft.value.revision,
			lesson.revision,
		);
		selection.value = undefined;
		await commit(r.draftRevision, t('authoring.structure.feedback.lessonDeleted'));
	});
}
async function mutate(action: () => Promise<void>, preserveMessage = false) {
	if (busy.value || conflict.value) return;
	busy.value = true;
	error.value = undefined;
	if (!preserveMessage) message.value = undefined;
	try {
		await action();
	} catch (cause) {
		fail(cause);
	} finally {
		busy.value = false;
	}
}
async function commit(revision: number, text: string) {
	const left = window.scrollX;
	const top = window.scrollY;
	replaceDraft({ ...draft.value, revision });
	await load(true);
	if (left || top) {
		await nextTick();
		window.scrollTo(left, top);
		window.requestAnimationFrame?.(() => window.scrollTo(left, top));
	}
	message.value = text;
}
function fail(cause: unknown) {
	if (cause instanceof APIProblemError && cause.status === 404) {
		markDraftUnavailable();
		return;
	}
	if (cause instanceof APIProblemError && cause.status === 409) {
		conflict.value = true;
		return;
	}
	error.value =
		cause instanceof APIProblemError && cause.status === 400
			? t('authoring.structure.feedback.saveInvalid')
			: t('authoring.structure.feedback.saveUnavailable');
}
</script>
