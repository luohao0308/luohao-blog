<script setup lang="ts">
import { formatDate, usePublishedArticles } from '~/composables/useArticles'

useHead({ title: '阅读排行' })

const { data, status, error, refresh } = await usePublishedArticles()
const ranking = computed(() =>
  [...(data.value ?? [])]
    .sort((a, b) => (b.view_count ?? 0) - (a.view_count ?? 0))
    .slice(0, 10),
)
</script>

<template>
  <div class="space-y-8">
    <PageHeader title="阅读排行" description="按 24h 去重后的阅读量排序的前十篇文章。" />

    <div v-if="error" role="alert" class="space-y-3 py-12 text-center">
      <p class="text-sm text-slate-600 dark:text-slate-400">排行榜加载失败，请稍后重试。</p>
      <button type="button" class="text-sm text-[#3c5d85] underline dark:text-blue-300" @click="refresh()">重新加载</button>
    </div>
    <p v-else-if="status === 'pending'" role="status" class="py-12 text-center text-sm text-slate-500">正在加载排行…</p>
    <ol v-else-if="ranking.length" class="space-y-2">
      <li v-for="(article, index) in ranking" :key="article.id">
        <NuxtLink
          :to="`/posts/${encodeURIComponent(article.slug)}`"
          class="flex items-baseline gap-4 rounded-xl border border-slate-200 px-4 py-3 transition-colors hover:border-[#3c5d85] dark:border-slate-800 dark:hover:border-blue-300"
        >
          <span
            class="w-8 shrink-0 text-center font-mono text-sm"
            :class="index < 3 ? 'text-[#3c5d85] dark:text-blue-300' : 'text-slate-400 dark:text-slate-500'"
          >{{ index + 1 }}</span>
          <span class="min-w-0 flex-1 break-words text-sm font-medium text-slate-800 dark:text-slate-100">{{ article.title }}</span>
          <span class="shrink-0 text-xs text-slate-400 dark:text-slate-500">{{ article.view_count ?? 0 }} 次阅读 · {{ formatDate(article.published_at || article.created_at) }}</span>
        </NuxtLink>
      </li>
    </ol>
    <p v-else class="py-16 text-center text-sm text-slate-500 dark:text-slate-400">还没有阅读数据。</p>
  </div>
</template>
