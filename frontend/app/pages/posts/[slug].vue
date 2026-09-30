<script setup lang="ts">
import { ARTICLE_STATUS_LABEL, formatDate, useArticleBySlug, usePublishedArticles } from '~/composables/useArticles'

const route = useRoute()
definePageMeta({ key: route => route.path })
const slug = computed(() => String(route.params.slug))

const { data, error } = await useArticleBySlug(slug.value)

if (error.value) {
  throw createError({ statusCode: error.value.statusCode === 404 ? 404 : 503, message: error.value.statusCode === 404 ? '文章不存在' : '文章暂时无法加载' })
}
if (!data.value) throw createError({ statusCode: 404, message: '文章不存在' })

// Non-null by the 404 guard above; the backend renderer owns the HTML.
const article = computed(() => data.value!)
const { data: published, error: navigationError } = await usePublishedArticles()
const adjacent = computed(() => {
  const list = published.value ?? []
  const index = list.findIndex(item => item.slug === slug.value)
  return index < 0 ? {} : { newer: list[index - 1], older: list[index + 1] }
})

// Report one view per page open. Fire-and-forget: the server deduplicates by
// client identity within 24h, and the counter the page shows is the one read
// during SSR (this visit shows up on the next look).
onMounted(() => {
  $fetch(`/api/v1/articles/${encodeURIComponent(slug.value)}/view`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  }).catch(() => {})
})

useHead({ title: article.value.title })
</script>

<template>
  <article class="space-y-6">
    <header class="space-y-5 border-b border-slate-200 pb-8 dark:border-slate-800">
      <NuxtLink to="/posts" class="text-xs text-[#3c5d85] hover:underline dark:text-blue-300">返回全部文章</NuxtLink>
      <h1 class="break-words text-3xl font-medium leading-relaxed">{{ article.title }}</h1>
      <div class="flex flex-wrap items-center gap-3 text-sm text-slate-500 dark:text-slate-400">
        <time>{{ formatDate(article.published_at || article.created_at) }}</time>
        <span aria-label="阅读量">{{ article.view_count }} 次阅读</span>
        <span
          v-if="article.status !== 2"
          class="rounded bg-amber-100 px-1.5 py-0.5 text-xs text-amber-700"
        >{{ ARTICLE_STATUS_LABEL[article.status] }}</span>
        <NuxtLink v-for="tag in article.tags" :key="tag" :to="`/tags/${encodeURIComponent(tag)}`" class="max-w-full break-words rounded bg-[#e7eee9] px-2 py-1 text-xs text-[#365f53] hover:underline dark:bg-emerald-950 dark:text-emerald-200">
          {{ tag }}
        </NuxtLink>
      </div>
    </header>

    <!-- eslint-disable-next-line vue/no-v-html -- trusted backend-rendered HTML -->
    <div class="markdown-body space-y-4 leading-7" v-html="article.content_html" />
    <nav v-if="adjacent.newer || adjacent.older" aria-label="相邻文章" class="grid gap-6 border-t border-slate-200 pt-8 sm:grid-cols-2 dark:border-slate-800">
      <div>
        <NuxtLink v-if="adjacent.newer" :to="`/posts/${encodeURIComponent(adjacent.newer.slug)}`" class="block space-y-2 hover:text-[#3c5d85] dark:hover:text-blue-300">
          <span class="text-xs text-slate-500 dark:text-slate-400">上一篇 · 较新</span>
          <p class="break-words text-sm leading-7">{{ adjacent.newer.title }}</p>
        </NuxtLink>
      </div>
      <div class="sm:text-right">
        <NuxtLink v-if="adjacent.older" :to="`/posts/${encodeURIComponent(adjacent.older.slug)}`" class="block space-y-2 hover:text-[#3c5d85] dark:hover:text-blue-300">
          <span class="text-xs text-slate-500 dark:text-slate-400">下一篇 · 较早</span>
          <p class="break-words text-sm leading-7">{{ adjacent.older.title }}</p>
        </NuxtLink>
      </div>
    </nav>
    <p v-if="navigationError" class="text-sm text-slate-500 dark:text-slate-400">相邻文章暂时无法加载。</p>
  </article>
</template>
