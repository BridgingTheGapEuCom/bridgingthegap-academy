<template>
  <section class="authoring-structure-module" :aria-labelledby="`authoring-module-${module.id}`">
    <header class="authoring-structure-module__header">
      <div>
        <p class="authoring-structure-module__position">Module {{ module.position + 1 }}</p>
        <h3 :id="`authoring-module-${module.id}`">{{ module.title }}</h3>
        <p v-if="module.description">{{ module.description }}</p>
        <p class="authoring-structure-module__key">Stable key: <code>{{ module.stable_key }}</code></p>
      </div>
      <div class="authoring-structure-module__actions" aria-label="Module actions">
        <BtgButton variant="secondary" :disabled="busy" @click="editingModule = !editingModule">{{ editingModule ? 'Close module edit' : 'Edit module' }}</BtgButton>
        <BtgButton variant="quiet" :disabled="busy || moduleIndex === 0" @click="$emit('move-module', -1)">Move module up</BtgButton>
        <BtgButton variant="quiet" :disabled="busy || moduleIndex === moduleCount - 1" @click="$emit('move-module', 1)">Move module down</BtgButton>
        <BtgButton variant="destructive" :disabled="busy || module.lessons.length > 0" @click="$emit('delete-module', module)">Delete empty module</BtgButton>
      </div>
    </header>

    <form v-if="editingModule" class="authoring-structure-module__form" @submit.prevent="saveModule">
      <BtgFormField label="Module title" required :error="moduleError" v-slot="{ controlId, describedBy, invalid }">
        <BtgTextInput :id="controlId" v-model="moduleForm.title" :aria-describedby="describedBy" :invalid="invalid" required />
      </BtgFormField>
      <BtgFormField label="Module description" v-slot="{ controlId, describedBy }">
        <textarea :id="controlId" v-model="moduleForm.description" :aria-describedby="describedBy" />
      </BtgFormField>
      <div class="authoring-structure-module__actions"><BtgButton type="submit" :disabled="busy || !moduleDirty">Save module</BtgButton></div>
    </form>

    <ol class="authoring-structure-module__lessons" :aria-label="`${module.title} lessons`">
      <li v-for="(lesson, lessonIndex) in module.lessons" :key="lesson.id" class="authoring-structure-lesson">
        <article>
          <p class="authoring-structure-lesson__position">Lesson {{ lessonIndex + 1 }}</p>
          <h4><RouterLink :to="authoringDraftLessonPath(draftId, lesson.id)">{{ lesson.title }}</RouterLink></h4>
          <p v-if="lesson.description">{{ lesson.description }}</p>
          <p class="authoring-structure-lesson__key">Stable key: <code>{{ lesson.stable_key }}</code></p>
          <div class="authoring-structure-lesson__actions" :aria-label="`${lesson.title} structure actions`">
            <BtgButton variant="quiet" :disabled="busy || lessonIndex === 0" @click="$emit('move-lesson', { lessonID: lesson.id, moduleID: module.id, position: lessonIndex - 1 })">Move lesson up</BtgButton>
            <BtgButton variant="quiet" :disabled="busy || lessonIndex === module.lessons.length - 1" @click="$emit('move-lesson', { lessonID: lesson.id, moduleID: module.id, position: lessonIndex + 1 })">Move lesson down</BtgButton>
            <BtgButton v-if="previousModule" variant="quiet" :disabled="busy" @click="$emit('move-lesson', { lessonID: lesson.id, moduleID: previousModule.id, position: previousModule.lessons.length })">Move to previous module</BtgButton>
            <BtgButton v-if="nextModule" variant="quiet" :disabled="busy" @click="$emit('move-lesson', { lessonID: lesson.id, moduleID: nextModule.id, position: 0 })">Move to next module</BtgButton>
            <BtgButton variant="destructive" :disabled="busy" @click="$emit('delete-lesson', lesson)">Delete lesson</BtgButton>
          </div>
        </article>
      </li>
    </ol>

    <form class="authoring-structure-module__form" @submit.prevent="createLesson">
      <h4>Add lesson</h4>
      <BtgFormField label="Lesson stable key" required description="A stable identifier that cannot later be changed." :error="lessonErrors.stableKey" v-slot="{ controlId, describedBy, invalid }">
        <BtgTextInput :id="controlId" v-model="lessonForm.stableKey" :aria-describedby="describedBy" :invalid="invalid" required />
      </BtgFormField>
      <BtgFormField label="Lesson title" required :error="lessonErrors.title" v-slot="{ controlId, describedBy, invalid }">
        <BtgTextInput :id="controlId" v-model="lessonForm.title" :aria-describedby="describedBy" :invalid="invalid" required />
      </BtgFormField>
      <BtgFormField label="Initial lesson description" required :error="lessonErrors.description" v-slot="{ controlId, describedBy, invalid }">
        <textarea :id="controlId" v-model="lessonForm.description" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" required />
      </BtgFormField>
      <BtgFormField label="Initial learning objectives" required description="Enter one objective per line." :error="lessonErrors.objectives" v-slot="{ controlId, describedBy, invalid }">
        <textarea :id="controlId" v-model="lessonForm.objectivesText" :aria-describedby="describedBy" :aria-invalid="invalid || undefined" required />
      </BtgFormField>
      <div class="authoring-structure-module__actions"><BtgButton type="submit" :disabled="busy">Create lesson</BtgButton></div>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { authoringDraftLessonPath, type AuthoringLessonCreate, type AuthoringLessonSummary, type AuthoringModuleSummary } from '../authoring/authoring'
import BtgButton from './BtgButton.vue'
import BtgFormField from './BtgFormField.vue'
import BtgTextInput from './BtgTextInput.vue'

const props = defineProps<{
  draftId: string
  module: AuthoringModuleSummary
  moduleIndex: number
  moduleCount: number
  previousModule?: AuthoringModuleSummary
  nextModule?: AuthoringModuleSummary
  busy: boolean
}>()

const emit = defineEmits<{
  'update-module': [module: AuthoringModuleSummary, patch: { title?: string; description?: string }]
  'move-module': [delta: -1 | 1]
  'delete-module': [module: AuthoringModuleSummary]
  'create-lesson': [moduleID: string, input: AuthoringLessonCreate]
  'move-lesson': [input: { lessonID: string; moduleID: string; position: number }]
  'delete-lesson': [lesson: AuthoringLessonSummary]
}>()

const editingModule = ref(false)
const moduleError = ref<string>()
const moduleForm = reactive({ title: props.module.title, description: props.module.description })
const lessonForm = reactive({ stableKey: '', title: '', description: '', objectivesText: '' })
const lessonErrors = reactive<Record<string, string | undefined>>({})
const moduleDirty = computed(() => moduleForm.title !== props.module.title || moduleForm.description !== props.module.description)

watch(() => props.module.revision, () => {
  moduleForm.title = props.module.title
  moduleForm.description = props.module.description
  moduleError.value = undefined
})

watch(() => props.module.lessons.length, () => {
  lessonForm.stableKey = ''
  lessonForm.title = ''
  lessonForm.description = ''
  lessonForm.objectivesText = ''
  for (const key of Object.keys(lessonErrors)) delete lessonErrors[key]
})

function saveModule() {
  moduleError.value = undefined
  if (!moduleForm.title.trim()) { moduleError.value = 'Enter a module title.'; return }
  if (!moduleDirty.value) return
  const patch: { title?: string; description?: string } = {}
  if (moduleForm.title !== props.module.title) patch.title = moduleForm.title
  if (moduleForm.description !== props.module.description) patch.description = moduleForm.description
  emit('update-module', props.module, patch)
}

function createLesson() {
  for (const key of Object.keys(lessonErrors)) delete lessonErrors[key]
  if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(lessonForm.stableKey) || lessonForm.stableKey.length < 3 || lessonForm.stableKey.length > 96) lessonErrors.stableKey = 'Use lowercase letters, numbers, and hyphens.'
  if (!lessonForm.title.trim()) lessonErrors.title = 'Enter a lesson title.'
  if (!lessonForm.description.trim()) lessonErrors.description = 'Enter an initial lesson description.'
  const objectives = lessonForm.objectivesText.split('\n').map((value) => value.trim()).filter(Boolean)
  if (!objectives.length) lessonErrors.objectives = 'Enter at least one learning objective.'
  if (Object.keys(lessonErrors).length) return
  emit('create-lesson', props.module.id, {
    expectedDraftRevision: 0,
    stableKey: lessonForm.stableKey,
    title: lessonForm.title,
    description: lessonForm.description,
    objectives,
    position: props.module.lessons.length,
  })
}
</script>
