<script setup lang="ts">
import { articleSeries, seriesPosition } from '~/data/articleSeries'
import { usePublishedArticles } from '~/composables/useArticles'
const props = defineProps<{ slug: string }>()
const { data, error, refresh } = await usePublishedArticles()
const membership = computed(() => articleSeries.filter(series => series.articleSlugs.includes(props.slug)))
const positions = computed(() => membership.value.flatMap(series => {
  const position = seriesPosition(series, data.value ?? [], props.slug)
  return position ? [{ series, ...position }] : []
}))
</script>

<template>
  <section v-if="membership.length && (error || positions.length)" aria-label="系列阅读" class="space-y-4 rounded-xl border border-[var(--border-default)] p-5">
    <p v-if="error" role="alert" class="text-sm text-[var(--text-muted)]">
      系列导航暂时无法加载。
      <button type="button" class="min-h-11 px-2 text-[var(--accent)] underline" @click="refresh()">重新加载</button>
    </p>
    <div v-for="item in error ? [] : positions" :key="item.series.slug" class="space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <NuxtLink :to="`/series/${item.series.slug}`" class="inline-flex min-h-11 items-center text-sm font-medium text-[var(--accent)]">{{ item.series.title }} →</NuxtLink>
        <span class="text-xs text-[var(--text-muted)]">第 {{ item.index + 1 }} / {{ item.total }} 篇</span>
      </div>
      <nav aria-label="系列前后篇" class="grid gap-4 sm:grid-cols-2">
        <NuxtLink v-if="item.previous" :to="`/posts/${encodeURIComponent(item.previous.slug)}`" class="min-w-0 space-y-1 py-2 hover:text-[var(--accent)]">
          <span class="text-xs text-[var(--text-muted)]">← 系列上一篇</span><p class="break-words text-sm leading-7">{{ item.previous.title }}</p>
        </NuxtLink>
        <NuxtLink v-if="item.next" :to="`/posts/${encodeURIComponent(item.next.slug)}`" class="min-w-0 space-y-1 py-2 hover:text-[var(--accent)] sm:col-start-2">
          <span class="text-xs text-[var(--text-muted)]">系列下一篇 →</span><p class="break-words text-sm leading-7">{{ item.next.title }}</p>
        </NuxtLink>
      </nav>
    </div>
  </section>
</template>
