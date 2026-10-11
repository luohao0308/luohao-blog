<script setup lang="ts">
import { searchPublishedArticles, usePublishedArticles, type Article } from '~/composables/useArticles'

const props = defineProps<{ tag?: string, categorySlug?: string }>()
const query = ref('')
const { data, status, error, refresh } = await usePublishedArticles()
const searchResults = ref<Article[] | null>(null)
const searchPending = ref(false)
const searchError = ref(false)
let searchRequest = 0
let searchTimer: ReturnType<typeof setTimeout> | undefined

watch(query, (value) => {
  if (searchTimer) {
    clearTimeout(searchTimer)
    searchTimer = undefined
  }
  const normalized = value.trim()
  if (!normalized) {
    searchRequest++
    searchResults.value = null
    searchError.value = false
    searchPending.value = false
    return
  }
  const request = ++searchRequest
  searchPending.value = true
  searchError.value = false
  searchTimer = setTimeout(async () => {
    try {
      const results = await searchPublishedArticles(normalized)
      if (request === searchRequest) searchResults.value = results
    } catch {
      // Keep the collection usable when ES is unavailable or the API is down.
      if (request === searchRequest) {
        searchError.value = true
        searchResults.value = null
      }
    } finally {
      if (request === searchRequest) searchPending.value = false
    }
  }, 300)
})
onBeforeUnmount(() => {
  if (searchTimer) clearTimeout(searchTimer)
})
const tags = computed(() => [...new Set((data.value ?? []).flatMap(article => article.tags ?? []))].sort())
const articles = computed(() => (searchResults.value ?? data.value ?? []).filter(article => {
  // tags 在 proto3 JSON 下可能整体缺省，所有取值点都要兜住空数组
  const tagList = article.tags ?? []
  const text = `${article.title} ${article.summary ?? ''} ${tagList.join(' ')}`.toLocaleLowerCase()
  return (!props.categorySlug || article.category?.slug === props.categorySlug)
    && (!props.tag || tagList.includes(props.tag))
    && text.includes(query.value.trim().toLocaleLowerCase())
}))
</script>

<template>
  <section class="space-y-6" aria-label="文章列表" :aria-busy="status === 'pending'">
    <div class="flex flex-col gap-5 border-y border-slate-200 py-5 sm:flex-row sm:items-center sm:justify-between dark:border-slate-800">
      <nav aria-label="文章标签" class="flex min-w-0 flex-wrap gap-2 text-xs">
        <NuxtLink to="/posts" :aria-current="!tag ? 'page' : undefined" class="rounded-full px-3.5 py-1.5" :class="!tag ? 'bg-[#3c5d85] text-white dark:bg-blue-300 dark:text-slate-950' : 'bg-slate-100 text-slate-600 dark:bg-slate-900 dark:text-slate-300'">全部</NuxtLink>
        <NuxtLink v-for="item in tags" :key="item" :to="`/tags/${encodeURIComponent(item)}`" :aria-current="tag === item ? 'page' : undefined" class="max-w-full break-words rounded-full px-3.5 py-1.5" :class="tag === item ? 'bg-[#3c5d85] text-white dark:bg-blue-300 dark:text-slate-950' : 'bg-slate-100 text-slate-600 hover:bg-[#e5def1] dark:bg-slate-900 dark:text-slate-300 dark:hover:bg-slate-800'">{{ item }}</NuxtLink>
      </nav>
      <div class="w-full shrink-0 sm:w-56">
        <label for="article-search" class="sr-only">搜索文章</label>
        <input id="article-search" v-model="query" type="search" placeholder="搜索文章" class="w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-800 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100">
      </div>
    </div>
    <div v-if="error" role="alert" class="space-y-3 py-12 text-center">
      <p class="text-sm text-slate-600 dark:text-slate-400">文章加载失败，请稍后重试。</p>
      <button type="button" class="text-sm text-[#3c5d85] underline dark:text-blue-300" @click="refresh()">重新加载</button>
    </div>
    <p v-else-if="status === 'pending' || searchPending" role="status" class="py-12 text-center text-sm text-slate-500">正在加载文章…</p>
    <template v-else>
      <p v-if="searchError" class="text-xs text-slate-500 dark:text-slate-400">搜索服务暂不可用，已显示本地匹配结果。</p>
      <p role="status" class="text-xs text-slate-500 dark:text-slate-400">{{ articles.length }} 篇文章</p>
      <div v-if="articles.length" class="grid auto-rows-fr gap-5 sm:grid-cols-2">
        <ArticleCard v-for="article in articles" :key="article.id" :article="article" />
      </div>
      <p v-else class="py-16 text-center text-sm text-slate-500 dark:text-slate-400">{{ query ? '没有找到匹配的文章。' : categorySlug ? '这个分类下暂无文章。' : tag ? '这个标签下暂无文章。' : '第一篇文章正在路上。' }}</p>
    </template>
  </section>
</template>
