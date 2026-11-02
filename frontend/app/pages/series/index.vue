<script setup lang="ts">
import { articleSeries, seriesArticles } from '~/data/articleSeries'
import { usePublishedArticles } from '~/composables/useArticles'
const { data, error, refresh } = await usePublishedArticles()
const series = computed(() => articleSeries.map(item => ({ ...item, articles: seriesArticles(item, data.value ?? []) })).filter(item => item.articles.length))
usePageSeo({ title: '文章系列', description: '按主题和阅读顺序，串起博客工程与 AI 交付实践。' })
</script>

<template>
  <div class="space-y-8">
    <PageHeader title="文章系列" description="沿着一条阅读路径，了解实现过程和取舍。">
      <template #eyebrow><NuxtLink to="/posts" class="detail-button">← 全部文章</NuxtLink></template>
    </PageHeader>
    <p v-if="error" role="alert" class="text-sm text-[var(--text-muted)]">
      系列文章暂时无法加载。
      <button type="button" class="min-h-11 px-2 text-[var(--accent)] underline" @click="refresh()">重新加载</button>
    </p>
    <div v-else-if="series.length" class="grid gap-5 sm:grid-cols-2">
      <NuxtLink v-for="item in series" :key="item.slug" :to="`/series/${item.slug}`" class="bento-card block min-w-0 space-y-4 rounded-xl border border-[var(--border-default)] p-6 hover:border-[var(--accent)]">
        <p class="text-xs text-[var(--text-muted)]">{{ item.articles.length }} 篇文章 · 按顺序阅读</p>
        <h2 class="text-xl font-medium">{{ item.title }}</h2>
        <p class="text-sm leading-7 text-[var(--text-muted)]">{{ item.description }}</p>
        <span class="inline-flex min-h-11 items-center text-sm text-[var(--accent)]">查看系列 →</span>
      </NuxtLink>
    </div>
    <p v-else class="text-sm text-[var(--text-muted)]">暂无已公开的系列文章。</p>
  </div>
</template>
