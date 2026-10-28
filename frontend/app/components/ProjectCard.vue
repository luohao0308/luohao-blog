<script setup lang="ts">
import { projectImages } from '~/data/editorial'
import type { Project } from '~/composables/usePortfolio'

const props = defineProps<{
  project: Project
  coverSrc?: string
  coverAlt?: string
}>()
const projectTo = computed(() => `/projects/${encodeURIComponent(props.project.slug)}`)
const openProject = useArticleExpansion('[data-project-detail]')
const editorialCover = computed(() => projectImages[props.project.slug])
const coverUrl = computed(() => props.coverSrc || assetUrl(props.project.cover_url) || editorialCover.value?.src || '')
const isPoster = computed(() => !props.coverSrc && !props.project.cover_url && editorialCover.value?.poster)
const alt = computed(() => props.coverAlt || (!props.project.cover_url && editorialCover.value?.alt) || `${props.project.name} 项目封面`)
const failedCover = ref('')
const coverImage = ref<HTMLImageElement | null>(null)
onMounted(() => {
  // An SSR image can fail before hydration attaches the error listener.
  if (coverImage.value?.complete && !coverImage.value.naturalWidth) failedCover.value = coverUrl.value
})
const showCover = computed(() => coverUrl.value && coverUrl.value !== failedCover.value)
</script>

<template>
  <article data-project-card class="editorial-card bento-card work-card" :class="{ 'has-cover': showCover, 'poster-card': showCover && isPoster, 'tone-sand': !project.featured }" @click.capture="openProject($event, projectTo)">
    <div v-if="showCover" class="editorial-cover">
      <img ref="coverImage" :src="coverUrl" :alt="alt" :width="isPoster ? 1280 : 800" :height="isPoster ? 720 : 600" loading="lazy" decoding="async" @error="failedCover = coverUrl">
    </div>
    <div class="editorial-body">
      <div class="editorial-meta">
        <span class="editorial-topic">{{ project.role }}</span>
        <span>{{ project.year }}</span>
      </div>
      <div class="editorial-copy">
        <h2 class="editorial-heading">
          <NuxtLink :to="projectTo" class="editorial-link">{{ project.name }}</NuxtLink>
        </h2>
        <p class="editorial-summary">{{ project.summary }}<span v-if="project.outcome"> · {{ project.outcome }}</span></p>
      </div>
      <div class="editorial-footer">
        <span class="editorial-stack" :title="(project.stack ?? []).join(' · ')">{{ (project.stack ?? []).join(' · ') }}</span>
        <span class="editorial-arrow" aria-hidden="true">↗</span>
      </div>
    </div>
  </article>
</template>
