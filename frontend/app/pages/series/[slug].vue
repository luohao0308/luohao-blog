<script setup lang="ts">
import { articleSeries, seriesArticles } from '~/data/articleSeries'
import { usePublishedArticles } from '~/composables/useArticles'
definePageMeta({ key: route => route.path })
const route = useRoute()
const series = articleSeries.find(item => item.slug === route.params.slug)
if (!series) throw createError({ statusCode: 404, message: '系列不存在' })
const { data, error, refresh } = await usePublishedArticles()
const articles = computed(() => seriesArticles(series, data.value ?? []))
const firstArticle = computed(() => articles.value[0])
const failedCover = ref(false)
const image = ref<HTMLImageElement | null>(null)
onMounted(() => {
  if (image.value?.complete && !image.value.naturalWidth) failedCover.value = true
})
usePageSeo({ title: `${series.title} · 文章系列`, description: series.description })
</script>

<template>
  <div data-series-detail class="space-y-8">
    <NuxtLink to="/series" class="detail-button">← 全部系列</NuxtLink>
    <img v-if="!failedCover" ref="image" class="series-hero" :src="series.cover" :alt="series.coverAlt" width="1056" height="596" decoding="async" @error="failedCover = true">
    <header class="space-y-3">
      <h1 tabindex="-1" class="break-words text-3xl font-bold tracking-tight focus:outline-none sm:text-4xl">{{ series.title }}</h1>
      <p class="text-sm leading-7 text-[var(--text-muted)]">{{ series.description }}</p>
    </header>
    <p v-if="error" role="alert" class="text-sm text-[var(--text-muted)]">
      系列文章暂时无法加载。
      <button type="button" class="min-h-11 px-2 text-[var(--accent)] underline" @click="refresh()">重新加载</button>
    </p>
    <section v-else-if="articles.length" aria-label="系列目录" class="space-y-6">
      <div class="series-reading-path">
        <div class="min-w-0 space-y-2">
          <h2>目录</h2>
          <p>共 {{ articles.length }} 篇 · {{ series.topics.join(' → ') }}</p>
        </div>
        <NuxtLink v-if="firstArticle" :to="`/posts/${encodeURIComponent(firstArticle.slug)}`" class="ui-button shrink-0">开始阅读 · 第 1 篇 →</NuxtLink>
      </div>
      <ol class="media-feed series-article-grid">
        <li v-for="(article, index) in articles" :key="article.slug" class="min-w-0 space-y-3">
          <p class="series-order">{{ String(index + 1).padStart(2, '0') }} / {{ articles.length }}</p>
          <ArticleCard :article="article" />
        </li>
      </ol>
    </section>
    <p v-else class="text-sm text-[var(--text-muted)]">本系列暂无已公开文章。</p>
  </div>
</template>

<style scoped>
.series-hero { display: block; width: 100%; height: auto; aspect-ratio: 16 / 9; object-fit: contain; border-radius: 16px; }
.series-reading-path { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 20px; padding: 24px; border-radius: 16px; background: var(--surface-hover); }
.series-reading-path h2 { color: var(--accent); font-size: 17px; font-weight: 700; }
.series-reading-path p { color: var(--text-muted); font-size: 13px; line-height: 1.7; overflow-wrap: anywhere; }
.series-order { color: var(--accent); font-size: 12px; font-weight: 600; font-variant-numeric: tabular-nums; }
@media (min-width: 1024px) { .series-article-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
@media (max-width: 640px) { .series-article-grid { grid-template-columns: minmax(0, 1fr); } }
</style>
