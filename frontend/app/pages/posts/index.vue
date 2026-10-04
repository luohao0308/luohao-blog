<script setup lang="ts">
import { usePublishedArticles } from '~/composables/useArticles'

useHead({ title: '文章' })

// Hot slot: the three most-read published articles, fetched through the same
// cached useAsyncData the collection below uses (one backend query total).
const { data: published } = await usePublishedArticles()
const hot = computed(() =>
  [...(published.value ?? [])]
    .sort((a, b) => (b.view_count ?? 0) - (a.view_count ?? 0))
    .slice(0, 3),
)
</script>

<template>
  <div class="space-y-9">
    <header class="space-y-4 py-5">
      <p class="text-xs text-[#3c5d85] dark:text-blue-300">Writing / 技术记录</p>
      <h1 class="text-3xl font-medium text-slate-800 dark:text-slate-100">文章</h1>
      <p class="max-w-xl text-sm leading-8 text-slate-600 dark:text-slate-400">从实际项目出发，记录 Go、全栈开发与 AI Agent 的实现、取舍和复盘。</p>
    </header>

    <section v-if="hot.length" aria-label="热门文章" class="space-y-4">
      <h2 class="text-sm font-medium text-slate-500 dark:text-slate-400">热门阅读</h2>
      <div class="grid gap-3 sm:grid-cols-3">
        <NuxtLink
          v-for="(item, index) in hot"
          :key="item.id"
          :to="`/posts/${encodeURIComponent(item.slug)}`"
          class="rounded-lg border border-slate-200 p-4 transition-shadow hover:shadow-md dark:border-slate-800"
        >
          <div class="flex items-baseline justify-between text-xs text-slate-400 dark:text-slate-500">
            <span>NO.{{ index + 1 }}</span>
            <span>{{ item.view_count ?? 0 }} 次阅读</span>
          </div>
          <h3 class="mt-2 line-clamp-2 text-sm font-medium">{{ item.title }}</h3>
        </NuxtLink>
      </div>
    </section>

    <ArticleCollection />
  </div>
</template>
