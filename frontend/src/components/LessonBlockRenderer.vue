<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { RenderableBlock } from '../lesson/content'
import { publishedAssetURL, type PublishedAssetDeliveryContext } from '../lesson/assets'
import LessonRichText from './LessonRichText.vue'
import LessonRichTextInlines from './LessonRichTextInlines.vue'
import LearnerKnowledgeCheck from './LearnerKnowledgeCheck.vue'
import type { PublishedAssessmentLearnerView } from '../courses/courses'
import type { LearnerAttemptSession } from '../courses/attempts'

const props = defineProps<{ block: RenderableBlock; publishedAssetContext?: PublishedAssetDeliveryContext; publishedAssessment?: PublishedAssessmentLearnerView; attemptSession?: LearnerAttemptSession }>()

const headingTag = computed(() => props.block.type === 'HEADING' ? `h${props.block.payload.level}` : 'h2')
const calloutLabel = computed(() => props.block.type === 'CALLOUT' ? ({ INFO: 'Information', NOTE: 'Note', WARNING: 'Warning', TIP: 'Tip' }[props.block.payload.kind]) : '')

const mediaFailed = ref(false)

watch(() => props.block, () => { mediaFailed.value = false }, { deep: true })

function assetURL(asset: { assetKey: string }, download = false): string | undefined {
  return props.publishedAssetContext ? publishedAssetURL(props.publishedAssetContext, asset, download) : undefined
}
</script>

<template>
  <section v-if="block.type === 'TEXT'" class="lesson-block lesson-block--text">
    <LessonRichText :content="block.payload.content" />
  </section>

  <component v-else-if="block.type === 'HEADING'" :is="headingTag" class="lesson-block lesson-block--heading">
    <LessonRichTextInlines :items="block.payload.content" />
  </component>

  <figure v-else-if="block.type === 'IMAGE'" class="lesson-block lesson-media" :class="{ 'lesson-media--unresolved': !assetURL(block.payload.asset) || mediaFailed }">
    <img v-if="assetURL(block.payload.asset) && !mediaFailed" class="lesson-media__element lesson-media__image" :src="assetURL(block.payload.asset)" :alt="block.payload.decorative ? '' : block.payload.altText" @error="mediaFailed = true" />
    <div v-else-if="block.payload.decorative" aria-hidden="true">Image asset unavailable</div>
    <div v-else role="img" :aria-label="block.payload.altText">Image asset unavailable</div>
    <figcaption v-if="block.payload.caption">{{ block.payload.caption }}</figcaption>
  </figure>

  <div v-else-if="block.type === 'VIDEO' || block.type === 'AUDIO'" class="lesson-block lesson-media" :class="{ 'lesson-media--unresolved': !assetURL(block.payload.asset) || mediaFailed }">
    <p><strong>{{ block.type === 'VIDEO' ? 'Video' : 'Audio' }}: {{ block.payload.title }}</strong></p>
    <video v-if="block.type === 'VIDEO' && assetURL(block.payload.asset) && !mediaFailed" class="lesson-media__element" controls preload="metadata" @error="mediaFailed = true">
      <source :src="assetURL(block.payload.asset)" />
    </video>
    <audio v-else-if="block.type === 'AUDIO' && assetURL(block.payload.asset) && !mediaFailed" class="lesson-media__element" controls preload="metadata" @error="mediaFailed = true">
      <source :src="assetURL(block.payload.asset)" />
    </audio>
    <p v-else>{{ block.type === 'VIDEO' ? 'Video' : 'Audio' }} asset unavailable.</p>
    <div class="lesson-media__transcript">
      <h3>Transcript</h3>
      <p v-if="block.payload.transcript">{{ block.payload.transcript }}</p>
      <p v-else-if="block.payload.transcriptAsset && assetURL(block.payload.transcriptAsset, true)"><a :href="assetURL(block.payload.transcriptAsset, true)">Download transcript</a></p>
      <p v-else>Transcript unavailable.</p>
      <p v-if="block.type === 'VIDEO' && assetURL(block.payload.captionsAsset, true)"><a :href="assetURL(block.payload.captionsAsset, true)">Download captions</a></p>
    </div>
  </div>

  <section v-else-if="block.type === 'CODE'" class="lesson-block lesson-block--code">
    <p v-if="block.payload.title || block.payload.language" class="lesson-block__label">{{ block.payload.title ?? 'Code' }}<template v-if="block.payload.language"> · {{ block.payload.language }}</template></p>
    <pre><code>{{ block.payload.code }}</code></pre>
  </section>

  <blockquote v-else-if="block.type === 'QUOTE'" class="lesson-block lesson-block--quote">
    <p>{{ block.payload.text }}</p>
    <footer v-if="block.payload.attribution || block.payload.sourceUrl">
      <cite v-if="block.payload.attribution">{{ block.payload.attribution }}</cite>
      <template v-if="block.payload.attribution && block.payload.sourceUrl"> · </template>
      <a v-if="block.payload.sourceUrl" :href="block.payload.sourceUrl">Source</a>
    </footer>
  </blockquote>

  <div v-else-if="block.type === 'CALLOUT'" class="lesson-block lesson-callout" :class="`lesson-callout--${block.payload.kind.toLowerCase()}`" role="note" :aria-label="calloutLabel">
    <p class="lesson-callout__kind">{{ calloutLabel }}</p>
    <p v-if="block.payload.title" class="lesson-callout__title">{{ block.payload.title }}</p>
    <LessonRichText :content="block.payload.content" />
  </div>

  <div
    v-else-if="block.type === 'TABLE'"
    class="lesson-block lesson-table-scroll"
    role="region"
    :aria-label="block.payload.caption ? `Scrollable table: ${block.payload.caption}` : 'Scrollable lesson table'"
    tabindex="0"
  >
    <table>
      <caption v-if="block.payload.caption">{{ block.payload.caption }}</caption>
      <thead><tr><th v-for="header in block.payload.headers" :key="header" scope="col">{{ header }}</th></tr></thead>
      <tbody><tr v-for="(row, rowIndex) in block.payload.rows" :key="rowIndex"><td v-for="(cell, cellIndex) in row" :key="cellIndex">{{ cell }}</td></tr></tbody>
    </table>
  </div>

  <section v-else-if="block.type === 'DOWNLOAD'" class="lesson-block lesson-download">
    <p class="lesson-block__label">Download</p>
    <p v-if="assetURL(block.payload.asset, true)"><a :href="assetURL(block.payload.asset, true)" download><strong>{{ block.payload.label }}</strong></a></p>
    <p v-else><strong>{{ block.payload.label }}</strong></p>
    <p v-if="block.payload.description">{{ block.payload.description }}</p>
    <p v-if="!assetURL(block.payload.asset, true)">Download unavailable.</p>
  </section>

  <LearnerKnowledgeCheck v-else-if="block.type === 'KNOWLEDGE_CHECK' && publishedAssessment" :assessment="publishedAssessment" :attempt-session="attemptSession" :block-key="block.key" />

  <div v-else-if="block.type === 'KNOWLEDGE_CHECK'" class="lesson-block lesson-knowledge-check" role="note" aria-label="Knowledge check">
    <p class="lesson-block__label">Knowledge check</p>
    <p>Interactive knowledge checks will be available when assessments are enabled.</p>
  </div>

  <hr v-else-if="block.type === 'DIVIDER'" class="lesson-block lesson-block--divider" />

  <section v-else class="lesson-block lesson-block--unsupported" role="alert">
    <p>This lesson block cannot be displayed safely.</p>
  </section>
</template>
