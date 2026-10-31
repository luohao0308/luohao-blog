<script setup lang="ts">
import { searchPublishedArticles, usePublishedArticles, type Article } from '~/composables/useArticles'

const props = defineProps<{ tag?: string, categorySlug?: string, inlineSearch?: boolean }>()
const collectionRoot = ref<HTMLElement | null>(null)
useArticleReveal(collectionRoot)
const query = ref('')
const selectedCategory = ref('')
const selectedTag = ref('')
const activeCategory = computed(() => props.categorySlug || selectedCategory.value)
const activeTag = computed(() => props.tag || selectedTag.value)
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
  searchRequest++
})
const tags = computed(() => [...new Set((data.value ?? []).flatMap(article => article.tags ?? []))].sort())
const categories = computed(() => {
  const values = new Map<string, string>()
  for (const article of data.value ?? []) {
    if (article.category) values.set(article.category.slug, article.category.name)
  }
  return [...values].map(([slug, name]) => ({ slug, name })).sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
})
const selectedCategoryName = computed(() => categories.value.find(item => item.slug === selectedCategory.value)?.name ?? selectedCategory.value)
const articles = computed(() => (searchResults.value ?? data.value ?? []).filter(article => {
  // tags 在 proto3 JSON 下可能整体缺省，所有取值点都要兜住空数组
  const tagList = article.tags ?? []
  const text = `${article.title} ${article.summary ?? ''} ${tagList.join(' ')}`.toLocaleLowerCase()
  return (!activeCategory.value || article.category?.slug === activeCategory.value)
    && (!activeTag.value || tagList.includes(activeTag.value))
    && (!searchError.value || text.includes(query.value.trim().toLocaleLowerCase()))
}))
</script>

<template>
  <section ref="collectionRoot" class="space-y-6" aria-label="文章列表" :aria-busy="status === 'pending' || searchPending">
    <InlineSearch v-if="inlineSearch" v-model="query" label="搜索文章">
      <template #heading><slot name="heading" /></template>
      <template v-if="$slots.actions" #actions><slot name="actions" /></template>
      <template v-if="categories.length || tags.length" #filters>
        <div class="space-y-3">
          <nav v-if="categories.length" aria-label="文章分类筛选" class="flex flex-wrap gap-2 text-xs">
            <button type="button" :aria-pressed="!selectedCategory" class="rounded-full px-3 py-1.5" :class="!selectedCategory ? 'bg-[#3c5d85] text-white dark:bg-blue-300 dark:text-slate-950' : 'bg-slate-100 text-slate-600 dark:bg-slate-900 dark:text-slate-300'" @click="selectedCategory = ''">全部分类</button>
            <button v-for="item in categories" :key="item.slug" type="button" :aria-pressed="selectedCategory === item.slug" class="max-w-full break-words rounded-full px-3 py-1.5" :class="selectedCategory === item.slug ? 'bg-[#3c5d85] text-white dark:bg-blue-300 dark:text-slate-950' : 'bg-slate-100 text-slate-600 dark:bg-slate-900 dark:text-slate-300'" @click="selectedCategory = item.slug">{{ item.name }}</button>
          </nav>
          <nav v-if="tags.length" aria-label="文章标签筛选" class="flex flex-wrap gap-2 text-xs">
            <button type="button" :aria-pressed="!selectedTag" class="rounded-full px-3 py-1.5" :class="!selectedTag ? 'bg-[#3c5d85] text-white dark:bg-blue-300 dark:text-slate-950' : 'bg-slate-100 text-slate-600 dark:bg-slate-900 dark:text-slate-300'" @click="selectedTag = ''">全部标签</button>
            <button v-for="item in tags" :key="item" type="button" :aria-pressed="selectedTag === item" class="max-w-full break-words rounded-full px-3 py-1.5" :class="selectedTag === item ? 'bg-[#3c5d85] text-white dark:bg-blue-300 dark:text-slate-950' : 'bg-slate-100 text-slate-600 dark:bg-slate-900 dark:text-slate-300'" @click="selectedTag = item">{{ item }}</button>
          </nav>
        </div>
      </template>
    </InlineSearch>
    <div v-if="inlineSearch && (query.trim() || selectedCategory || selectedTag)" class="flex flex-wrap gap-2 text-xs" aria-label="当前筛选">
      <button v-if="query.trim()" type="button" class="max-w-full break-words rounded-full bg-slate-100 px-3 py-1.5 text-slate-600 dark:bg-slate-800 dark:text-slate-300" :aria-label="`清除搜索：${query.trim()}`" @click="query = ''">搜索：{{ query.trim() }} ×</button>
      <button v-if="selectedCategory" type="button" class="max-w-full break-words rounded-full bg-slate-100 px-3 py-1.5 text-slate-600 dark:bg-slate-800 dark:text-slate-300" :aria-label="`清除分类：${selectedCategoryName}`" @click="selectedCategory = ''">分类：{{ selectedCategoryName }} ×</button>
      <button v-if="selectedTag" type="button" class="max-w-full break-words rounded-full bg-slate-100 px-3 py-1.5 text-slate-600 dark:bg-slate-800 dark:text-slate-300" :aria-label="`清除标签：${selectedTag}`" @click="selectedTag = ''">标签：{{ selectedTag }} ×</button>
    </div>
    <div v-if="!inlineSearch" class="flex flex-col gap-5 border-y border-slate-200 py-5 sm:flex-row sm:items-center sm:justify-between dark:border-slate-800">
      <nav aria-label="文章标签" class="flex min-w-0 flex-wrap gap-2 text-xs">
        <NuxtLink to="/posts" :aria-current="!tag && !categorySlug ? 'page' : undefined" class="rounded-full px-3.5 py-1.5" :class="!tag && !categorySlug ? 'bg-[#3c5d85] text-white dark:bg-blue-300 dark:text-slate-950' : 'bg-slate-100 text-slate-600 dark:bg-slate-900 dark:text-slate-300'">全部</NuxtLink>
        <NuxtLink v-for="item in tags" :key="item" :to="`/tags/${encodeURIComponent(item)}`" :aria-current="tag === item ? 'page' : undefined" class="max-w-full break-words rounded-full px-3.5 py-1.5" :class="tag === item ? 'bg-[#3c5d85] text-white dark:bg-blue-300 dark:text-slate-950' : 'bg-slate-100 text-slate-600 dark:bg-slate-900 dark:text-slate-300'">{{ item }}</NuxtLink>
      </nav>
      <div class="w-full shrink-0 sm:w-56">
        <label for="article-search" class="sr-only">搜索文章</label>
        <input id="article-search" v-model="query" type="search" placeholder="搜索文章" class="ui-field w-full text-sm">
      </div>
    </div>
    <div v-if="error" role="alert" class="space-y-3 py-12 text-center">
      <p class="text-sm text-slate-600 dark:text-slate-400">文章加载失败，请稍后重试。</p>
      <button type="button" class="text-sm text-[#3c5d85] underline dark:text-blue-300" @click="refresh()">重新加载</button>
    </div>
    <p v-else-if="status === 'pending'" role="status" class="py-12 text-center text-sm text-slate-500">正在加载文章…</p>
    <template v-else>
      <p v-if="searchPending" role="status" class="text-sm text-[var(--text-muted)]">正在搜索，结果即将更新…</p>
      <p v-if="searchError" class="text-xs text-slate-500 dark:text-slate-400">搜索服务暂不可用，已显示本地匹配结果。</p>
      <p role="status" class="text-xs text-slate-500 dark:text-slate-400">{{ articles.length }} 篇文章</p>
      <div v-if="articles.length" class="media-feed">
        <ArticleCard v-for="article in articles" :key="article.id" :article="article" data-article-reveal />
      </div>
      <p v-else class="py-16 text-center text-sm text-slate-500 dark:text-slate-400">{{ query ? '没有找到匹配的文章。' : activeCategory ? '这个分类下暂无文章。' : activeTag ? '这个标签下暂无文章。' : '第一篇文章正在路上。' }}</p>
    </template>
  </section>
</template>
