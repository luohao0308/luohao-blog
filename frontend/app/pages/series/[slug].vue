<script setup lang="ts">
import { articleSeries, seriesArticles } from '~/data/articleSeries'
import { usePublishedArticles } from '~/composables/useArticles'
definePageMeta({ key: route => route.path })
const route = useRoute()
const series = articleSeries.find(item => item.slug === route.params.slug)
if (!series) throw createError({ statusCode: 404, message: '系列不存在' })
const { data, error, refresh } = await usePublishedArticles()
const articles = computed(() => seriesArticles(series, data.value ?? []))
usePageSeo({ title: `${series.title} · 文章系列`, description: series.description })
</script>

<template>
  <div class="space-y-8">
    <PageHeader :title="series.title" :description="series.description">
      <template #eyebrow><NuxtLink to="/series" class="detail-button">← 全部系列</NuxtLink></template>
    </PageHeader>
    <p v-if="error" role="alert" class="text-sm text-[var(--text-muted)]">
      系列文章暂时无法加载。
      <button type="button" class="min-h-11 px-2 text-[var(--accent)] underline" @click="refresh()">重新加载</button>
    </p>
    <section v-else-if="articles.length" aria-label="系列目录" class="space-y-4">
      <p class="text-sm text-[var(--text-muted)]">共 {{ articles.length }} 篇 · 建议按以下顺序阅读</p>
      <ol class="divide-y divide-[var(--border-default)]">
        <li v-for="(article, index) in articles" :key="article.slug">
          <NuxtLink :to="`/posts/${encodeURIComponent(article.slug)}`" class="flex min-h-11 gap-4 py-6 hover:text-[var(--accent)]">
            <span class="shrink-0 text-sm text-[var(--text-muted)]" aria-hidden="true">{{ String(index + 1).padStart(2, '0') }}</span>
            <div class="min-w-0 space-y-2">
              <h2 class="break-words text-lg font-medium">{{ article.title }}</h2>
              <p class="break-words text-sm leading-7 text-[var(--text-muted)]">{{ article.summary }}</p>
            </div>
          </NuxtLink>
        </li>
      </ol>
    </section>
    <p v-else class="text-sm text-[var(--text-muted)]">本系列暂无已公开文章。</p>
  </div>
</template>
