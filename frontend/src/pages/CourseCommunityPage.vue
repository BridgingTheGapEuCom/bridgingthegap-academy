<template>
  <BtgPageContainer as="section" class="community-page" aria-labelledby="community-title">
    <RouterLink :to="coursePath(courseId())" class="community-page__back">Back to course</RouterLink>
    <div v-if="state.kind === 'loading'" role="status"><h1 id="community-title">Loading discussions…</h1></div>
    <div v-else-if="state.kind === 'unavailable'" role="alert"><h1 id="community-title">Discussions unavailable</h1><p>We couldn’t load discussions right now.</p><BtgButton variant="secondary" @click="load">Try again</BtgButton></div>
    <template v-else>
      <header><h1 id="community-title" ref="heading" tabindex="-1">Course discussions</h1><p>Discuss this course with other participants.</p><RouterLink v-if="canModerate" :to="communityModerationPath(courseId())">Moderation view</RouterLink></header>
      <section v-if="state.mode === 'DISABLED'" class="community-page__empty" aria-labelledby="community-disabled"><h2 id="community-disabled">Discussions are unavailable</h2><p>Community discussions are not enabled for this course.</p></section>
      <template v-else>
        <form class="community-form" @submit.prevent="createThread">
          <h2>Start a discussion</h2>
          <label for="community-thread-title">Title</label><input id="community-thread-title" v-model="title" required maxlength="240" :disabled="creating">
          <label for="community-thread-body">Opening message</label><textarea id="community-thread-body" v-model="body" required maxlength="20000" rows="5" :disabled="creating" />
          <p v-if="formError" class="community-error" role="alert">{{ formError }}</p>
          <BtgButton type="submit" :disabled="creating">{{ creating ? 'Creating…' : 'Start discussion' }}</BtgButton>
        </form>
        <section aria-labelledby="community-threads-title"><h2 id="community-threads-title">Discussions</h2>
          <p v-if="state.page.threads.length === 0" class="community-page__empty">No discussions yet. Start one to begin the conversation.</p>
          <ol v-else class="community-threads"><li v-for="thread in state.page.threads" :key="thread.threadId"><RouterLink :to="communityThreadPath(courseId(), thread.threadId)">{{ thread.title }}</RouterLink><p>Participant · {{ thread.postCount }} {{ thread.postCount === 1 ? 'post' : 'posts' }} · <time :datetime="thread.updatedAt">{{ formatDate(thread.updatedAt) }}</time></p></li></ol>
          <nav v-if="state.page.total > state.page.limit" aria-label="Discussion pages"><BtgButton variant="secondary" :disabled="state.page.offset === 0" @click="changePage(-1)">Previous</BtgButton><BtgButton variant="secondary" :disabled="state.page.offset + state.page.limit >= state.page.total" @click="changePage(1)">Next</BtgButton></nav>
        </section>
      </template>
    </template>
  </BtgPageContainer>
</template>
<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { APIProblemError } from '../api/client'
import BtgButton from '../components/BtgButton.vue'
import BtgPageContainer from '../components/BtgPageContainer.vue'
import { communityPath, communityThreadPath, communityModerationPath, createCommunityThread, getCourseCommunity, getCommunityModerationProbe, listCommunityThreads, type CommunityThreadPage } from '../courses/community'
import { publishedCoursePath as coursePath } from '../courses/courses'
type State={kind:'loading'}|{kind:'unavailable'}|{kind:'ready',mode:'ENABLED'|'DISABLED',page:CommunityThreadPage}
const route=useRoute(),router=useRouter(),state=ref<State>({kind:'loading'}),title=ref(''),body=ref(''),formError=ref(''),creating=ref(false),heading=ref<HTMLElement|null>(null),canModerate=ref(false);let active=true,generation=0
const courseId=()=>typeof route.params.courseId==='string'?route.params.courseId:''
watch(()=>route.params.courseId,()=>{void load()},{immediate:true});onBeforeUnmount(()=>{active=false;generation++})
async function load(offset=0){const id=courseId(),request=++generation;state.value={kind:'loading'};canModerate.value=false;try{const meta=await getCourseCommunity(id);const page=meta.mode==='ENABLED'?await listCommunityThreads(id,offset):{threads:[],total:0,limit:20,offset:0};if(!active||request!==generation||id!==courseId())return;state.value={kind:'ready',mode:meta.mode,page};void getCommunityModerationProbe(id).then(x=>{if(active&&request===generation)canModerate.value=x.canModerate}).catch(()=>{})}catch{if(active&&request===generation)state.value={kind:'unavailable'}}}
function changePage(delta:number){if(state.value.kind==='ready')void load(Math.max(0,state.value.page.offset+delta*state.value.page.limit))}
async function createThread(){if(creating.value)return;creating.value=true;formError.value='';try{const thread=await createCommunityThread(courseId(),title.value,body.value);if(active)await router.push(communityThreadPath(courseId(),thread.threadId))}catch(error){if(error instanceof APIProblemError&&error.problem?.code==='community_disabled'){formError.value='Discussions were disabled. Your message has not been sent.';void load()}else formError.value='Your discussion could not be created. Please try again.'}finally{creating.value=false}}
function formatDate(value:string){return new Intl.DateTimeFormat('en',{dateStyle:'medium',timeStyle:'short'}).format(new Date(value))}
</script>
