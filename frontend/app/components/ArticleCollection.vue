<script setup lang="ts">
import { usePublishedArticles } from '~/composables/useArticles'

const props = defineProps<{ tag?: string }>()
const query = ref('')
const { data, status, error, refresh } = await usePublishedArticles()
const tags = computed(() => [...new Set((data.value ?? []).flatMap(article => article.tags))].sort())
const articles = computed(() => (data.value ?? []).filter(article => {
  const text = `${article.title} ${article.summary} ${article.tags.join(' ')}`.toLocaleLowerCase()
  return (!props.tag || article.tags.includes(props.tag)) && text.includes(query.value.trim().toLocaleLowerCase())
}))
</script>

<template>
  <section class="space-y-6" aria-label="文章列表" :aria-busy="status === 'pending'">
    <div class="flex flex-col gap-5 border-y border-slate-200 py-5 sm:flex-row sm:items-center sm:justify-between dark:border-slate-800">
      <nav aria-label="文章标签" class="flex min-w-0 flex-wrap gap-2 text-xs">
        <NuxtLink to="/posts" :aria-current="!tag ? 'page' : undefined" class="rounded-md px-3 py-2" :class="!tag ? 'bg-[#3c5d85] text-white dark:bg-blue-300 dark:text-slate-950' : 'bg-slate-100 text-slate-600 dark:bg-slate-900 dark:text-slate-300'">全部</NuxtLink>
        <NuxtLink v-for="item in tags" :key="item" :to="`/tags/${encodeURIComponent(item)}`" :aria-current="tag === item ? 'page' : undefined" class="max-w-full break-words rounded-md px-3 py-2" :class="tag === item ? 'bg-[#3c5d85] text-white dark:bg-blue-300 dark:text-slate-950' : 'bg-slate-100 text-slate-600 hover:bg-[#e5def1] dark:bg-slate-900 dark:text-slate-300 dark:hover:bg-slate-800'">{{ item }}</NuxtLink>
      </nav>
      <div class="w-full shrink-0 sm:w-56">
        <label for="article-search" class="sr-only">搜索文章</label>
        <input id="article-search" v-model="query" type="search" placeholder="搜索文章" class="w-full rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-800 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100">
      </div>
    </div>
    <div v-if="error" role="alert" class="space-y-3 py-12 text-center">
      <p class="text-sm text-slate-600 dark:text-slate-400">文章加载失败，请稍后重试。</p>
      <button type="button" class="text-sm text-[#3c5d85] underline dark:text-blue-300" @click="refresh()">重新加载</button>
    </div>
    <p v-else-if="status === 'pending'" role="status" class="py-12 text-center text-sm text-slate-500">正在加载文章…</p>
    <template v-else>
      <p role="status" class="text-xs text-slate-500 dark:text-slate-400">{{ articles.length }} 篇文章</p>
      <div v-if="articles.length" class="grid auto-rows-fr gap-5 sm:grid-cols-2">
        <ArticleCard v-for="article in articles" :key="article.id" :article="article" />
      </div>
      <p v-else class="py-16 text-center text-sm text-slate-500 dark:text-slate-400">{{ query ? '没有找到匹配的文章。' : tag ? '这个标签下暂无文章。' : '第一篇文章正在路上。' }}</p>
    </template>
  </section>
</template>
