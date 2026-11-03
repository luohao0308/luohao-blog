<script setup lang="ts">
import type { ArticleSeries } from '~/data/articleSeries'
const props = defineProps<{ series: ArticleSeries, count: number }>()
const destination = computed(() => `/series/${encodeURIComponent(props.series.slug)}`)
const openSeries = useArticleExpansion('[data-series-detail]')
const failedCover = ref(false)
const image = ref<HTMLImageElement | null>(null)
onMounted(() => {
  if (image.value?.complete && !image.value.naturalWidth) failedCover.value = true
})
</script>

<template>
  <article data-series-card class="series-card bento-card" @click.capture="openSeries($event, destination)">
    <img v-if="!failedCover" ref="image" :src="series.cover" :alt="series.coverAlt" width="528" height="298" loading="lazy" decoding="async" @error="failedCover = true">
    <div class="series-card-body">
      <h2><NuxtLink :to="destination" class="series-card-link">{{ series.title }}</NuxtLink></h2>
      <p>{{ count }} 篇文章 · 按顺序阅读</p>
      <p class="series-card-description">{{ series.description }}</p>
      <p>{{ series.topics.join(' / ') }}</p>
      <span class="series-card-action">查看目录 <span aria-hidden="true">↗</span></span>
    </div>
  </article>
</template>

<style scoped>
.series-card { position: relative; min-width: 0; overflow: hidden; border-radius: 16px; background: var(--surface-input); }
.series-card > img { display: block; width: 100%; height: auto; aspect-ratio: 16 / 9; object-fit: contain; }
.series-card-body { display: flex; flex-direction: column; gap: 12px; padding: 22px; }
h2 { font-size: 17px; line-height: 1.5; font-weight: 700; overflow-wrap: anywhere; }
.series-card-link::after { content: ''; position: absolute; inset: 0; }
.series-card-link:focus-visible { outline: none; }
.series-card-link:focus-visible::after { outline: 2px solid var(--focus-ring); outline-offset: -5px; border-radius: 16px; }
p { color: var(--text-muted); font-size: 12px; line-height: 1.7; overflow-wrap: anywhere; }
.series-card-description { font-size: 14px; }
.series-card-action { display: flex; justify-content: space-between; align-items: center; min-height: 32px; color: var(--accent); font-size: 13px; font-weight: 600; }
.series-card-action > span { font-size: 22px; }
</style>
