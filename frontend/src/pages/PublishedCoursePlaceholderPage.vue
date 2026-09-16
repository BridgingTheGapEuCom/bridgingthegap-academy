<template>
  <BtgPageContainer as="section" class="courses-page" width="reading" aria-labelledby="published-course-title">
    <div class="courses-page__state">
      <h1 id="published-course-title">Course reader coming soon</h1>
      <p>This published course is available in the catalog. Its learner reader will be added next.</p>
      <RouterLink to="/courses">Browse courses</RouterLink>
    </div>
  </BtgPageContainer>
</template>

<script setup lang="ts">
import BtgPageContainer from '../components/BtgPageContainer.vue'
</script>
