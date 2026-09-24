<template>
  <div class="course-package-export">
    <BtgButton :disabled="busy" @click="download">{{ busy ? 'Preparing export…' : `Export version ${version}` }}</BtgButton>
    <p v-if="error" class="course-package-export__error" role="alert">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import BtgButton from './BtgButton.vue'

const props = defineProps<{ courseId: string; version: string }>()
const busy = ref(false)
const error = ref<string>()

async function download() {
  if (busy.value) return
  busy.value = true
  error.value = undefined
  try {
    const response = await fetch(`/api/courses/by-id/${encodeURIComponent(props.courseId)}/versions/${encodeURIComponent(props.version)}/export`, { credentials: 'same-origin' })
    if (!response.ok || !response.headers.get('Content-Type')?.toLowerCase().startsWith('application/zip')) throw new Error('export failed')
    const blob = await response.blob()
    if (blob.size === 0) throw new Error('empty export')
    const anchor = document.createElement('a')
    anchor.href = URL.createObjectURL(blob)
    anchor.download = downloadFilename(response.headers.get('Content-Disposition'))
    anchor.click()
    URL.revokeObjectURL(anchor.href)
  } catch {
    error.value = 'We couldn’t export this course package right now. Please try again.'
  } finally { busy.value = false }
}

function downloadFilename(value: string | null): string {
  const match = value?.match(/filename="?([^";]+)"?/i)
  return match?.[1]?.trim() || `course-${props.version}.btg-course.zip`
}
</script>
