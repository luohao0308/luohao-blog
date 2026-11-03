<script setup lang="ts">
import { articleSeries, seriesArticles } from '~/data/articleSeries'
import { usePublishedArticles } from '~/composables/useArticles'
const { data, error, refresh } = await usePublishedArticles()
const series = computed(() => articleSeries.map(item => ({ ...item, articles: seriesArticles(item, data.value ?? []) })).filter(item => item.articles.length))
const entryArticles = computed(() => ['article-cas-revisions-case-study', 'forge-harness-project'].flatMap(slug => {
  const article = (data.value ?? []).find(item => item.slug === slug && item.status === 2)
  return article ? [article] : []
}))
usePageSeo({ title: '文章系列', description: '博客工程与 AI 开发相关文章，按主题分组。' })
</script>

<template>
  <div class="series-page space-y-8">
    <header class="space-y-4">
      <h1 class="text-3xl font-bold tracking-tight sm:text-[44px]">系列阅读</h1>
    </header>
    <p v-if="error" role="alert" class="text-sm text-[var(--text-muted)]">
      系列文章暂时无法加载。
      <button type="button" class="min-h-11 px-2 text-[var(--accent)] underline" @click="refresh()">重新加载</button>
    </p>
    <div v-else-if="series.length" class="grid items-start gap-6 sm:grid-cols-2">
      <SeriesCard v-for="item in series" :key="item.slug" :series="item" :count="item.articles.length" />
    </div>
    <p v-else class="text-sm text-[var(--text-muted)]">暂无已公开的系列文章。</p>
    <section v-if="!error && entryArticles.length" class="space-y-5" aria-label="系列阅读起点">
      <h2 class="text-xl font-bold">推荐文章</h2>
      <div class="media-feed series-entry-articles">
        <ArticleCard v-for="article in entryArticles" :key="article.slug" :article="article" />
      </div>
    </section>
  </div>
</template>

<style scoped>
@media (max-width: 640px) { .series-entry-articles { grid-template-columns: minmax(0, 1fr); } }
</style>
