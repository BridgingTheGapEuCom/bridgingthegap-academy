<script setup lang="ts">
import type { RichText } from '../lesson/content'
import LessonRichTextInlines from './LessonRichTextInlines.vue'

defineProps<{ content: RichText }>()
</script>

<template>
  <template v-for="(node, index) in content.nodes" :key="index">
    <p v-if="node.type === 'paragraph'">
      <LessonRichTextInlines :items="node.content" />
    </p>
    <ul v-else-if="node.type === 'bullet_list'">
      <li v-for="(item, itemIndex) in node.items" :key="itemIndex"><LessonRichTextInlines :items="item" /></li>
    </ul>
    <ol v-else-if="node.type === 'ordered_list'">
      <li v-for="(item, itemIndex) in node.items" :key="itemIndex"><LessonRichTextInlines :items="item" /></li>
    </ol>
    <pre v-else-if="node.type === 'code_block'" class="lesson-rich-text__code"><code>{{ node.text }}</code></pre>
  </template>
</template>
