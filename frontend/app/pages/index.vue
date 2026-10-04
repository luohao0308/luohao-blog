<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { projects } from '~/data/projects'
import { formatDate, searchPublishedArticles, useArticleList, usePublishedArticles } from '~/composables/useArticles'

const { data } = useArticleList({ pageSize: 3 })

useHead({ title: '首页' })

const socials = [
  { label: 'GitHub', href: 'https://github.com/luohao0308', text: 'GitHub' },
  { label: '邮箱', href: 'mailto:hello@example.com', text: 'Email' },
]

const featuredProjects = projects.filter((p) => p.featured)

const searchQuery = ref('')
const searchResults = ref<Awaited<ReturnType<typeof searchPublishedArticles>>>([])
const searchLoading = ref(false)
const searchError = ref('')
const { data: allArticles } = await usePublishedArticles()

const stats = computed(() => {
  const articles = allArticles.value ?? []
  const tags = new Set(articles.flatMap(a => a.tags ?? []))
  return {
    articleCount: articles.length,
    projectCount: projects.length,
    tagCount: tags.size,
    latestUpdated: articles[0]?.updated_at,
  }
})

const hotArticles = computed(() =>
  [...(allArticles.value ?? [])]
    .sort((a, b) => (b.view_count ?? 0) - (a.view_count ?? 0))
    .slice(0, 5),
)

watch(searchQuery, async (value) => {
  if (!value?.trim()) {
    searchResults.value = []
    return
  }
  searchLoading.value = true
  searchError.value = ''
  try {
    searchResults.value = await searchPublishedArticles(value)
  } catch {
    searchResults.value = []
    searchError.value = '搜索暂时不可用，请稍后再试。'
  } finally {
    searchLoading.value = false
  }
})
</script>

<template>
  <div class="space-y-14">
    <!-- hero -->
    <section class="space-y-4">
      <h1 class="text-4xl font-bold tracking-tight">
        你好，我是 <span class="text-blue-600 dark:text-blue-400">luohao</span> 👋
      </h1>
      <p class="max-w-2xl text-lg text-slate-600 dark:text-slate-400">
        全栈工程师，关注 Go 与 AI Agent 的工程实践。这里记录我写代码的思考、踩坑与复盘——这个网站本身也是其中的作品。
      </p>
      <div class="flex flex-wrap items-center gap-3">
        <NuxtLink
          to="/posts"
          class="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-slate-700 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-200"
        >
          阅读文章
        </NuxtLink>
        <NuxtLink
          to="/projects"
          class="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium transition-colors hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-900"
        >
          看看项目
        </NuxtLink>
        <a
          v-for="s in socials"
          :key="s.label"
          :href="s.href"
          target="_blank"
          rel="noopener"
          class="text-sm text-slate-500 underline-offset-4 hover:underline dark:text-slate-400"
        >{{ s.text }}</a>
      </div>

      <div class="max-w-2xl rounded-xl border border-slate-200 p-4 dark:border-slate-800">
        <label for="home-search" class="mb-2 block text-sm font-medium">搜索文章</label>
        <input
          id="home-search"
          v-model="searchQuery"
          type="search"
          placeholder="输入标题、摘要或关键词"
          class="w-full rounded-lg border border-slate-300 bg-transparent px-3 py-2 text-sm outline-none transition focus:border-blue-500 dark:border-slate-700"
        >
        <p v-if="searchLoading" class="mt-3 text-sm text-slate-500">正在搜索...</p>
        <p v-else-if="searchError" class="mt-3 text-sm text-red-600 dark:text-red-400">{{ searchError }}</p>
        <div v-else-if="searchQuery.trim() && !searchResults.length" class="mt-3 text-sm text-slate-500">没有找到相关文章。</div>
        <div v-else-if="searchResults.length" class="mt-3 space-y-2">
          <NuxtLink
            v-for="article in searchResults.slice(0, 5)"
            :key="article.id"
            :to="`/posts/${article.slug}`"
            class="block rounded-md px-2 py-1.5 text-sm hover:bg-slate-100 dark:hover:bg-slate-800"
          >{{ article.title }}</NuxtLink>
        </div>
      </div>

      <div class="grid gap-3 sm:grid-cols-4">
        <div class="rounded-lg border border-slate-200 p-4 dark:border-slate-800">
          <div class="text-xs text-slate-500">文章数</div>
          <div class="mt-1 text-2xl font-bold">{{ stats.articleCount }}</div>
        </div>
        <div class="rounded-lg border border-slate-200 p-4 dark:border-slate-800">
          <div class="text-xs text-slate-500">项目数</div>
          <div class="mt-1 text-2xl font-bold">{{ stats.projectCount }}</div>
        </div>
        <div class="rounded-lg border border-slate-200 p-4 dark:border-slate-800">
          <div class="text-xs text-slate-500">标签数</div>
          <div class="mt-1 text-2xl font-bold">{{ stats.tagCount }}</div>
        </div>
        <div class="rounded-lg border border-slate-200 p-4 dark:border-slate-800">
          <div class="text-xs text-slate-500">最近更新</div>
          <div class="mt-2 text-sm font-medium">{{ formatDate(stats.latestUpdated) || '暂无' }}</div>
        </div>
      </div>
    </section>

    <!-- featured articles -->
    <section class="space-y-4">
      <div class="flex items-baseline justify-between">
        <h2 class="text-xl font-bold tracking-tight">最新文章</h2>
        <NuxtLink to="/posts" class="text-sm text-blue-600 hover:underline dark:text-blue-400">全部文章 →</NuxtLink>
      </div>
      <div v-if="data?.articles?.length" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <NuxtLink
          v-for="a in data.articles"
          :key="a.id"
          :to="`/posts/${a.slug}`"
          class="rounded-lg border border-slate-200 p-4 transition-shadow hover:shadow-md dark:border-slate-800"
        >
          <div class="flex items-center justify-between gap-2 text-xs text-slate-500">
            <time>{{ formatDate(a.published_at || a.created_at) }}</time>
            <span>{{ a.tags?.[0] ?? '随笔' }}</span>
          </div>
          <h3 class="mt-2 font-semibold">{{ a.title }}</h3>
          <p class="mt-1 line-clamp-2 text-sm text-slate-600 dark:text-slate-400">
            {{ a.summary || a.content_md?.slice(0, 60) }}
          </p>
        </NuxtLink>
      </div>
      <p v-else class="text-sm text-slate-500">第一篇文章正在路上。</p>
    </section>

    <!-- popular articles -->
    <section class="space-y-4">
      <div class="flex items-baseline justify-between">
        <h2 class="text-xl font-bold tracking-tight">热门文章</h2>
        <NuxtLink to="/posts" class="text-sm text-blue-600 hover:underline dark:text-blue-400">浏览全部 →</NuxtLink>
      </div>
      <div v-if="hotArticles.length" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <NuxtLink
          v-for="article in hotArticles"
          :key="article.id"
          :to="`/posts/${article.slug}`"
          class="rounded-lg border border-slate-200 p-4 transition-shadow hover:shadow-md dark:border-slate-800"
        >
          <h3 class="line-clamp-2 font-medium">{{ article.title }}</h3>
          <p class="mt-2 text-xs text-slate-500">{{ article.view_count ?? 0 }} 次阅读</p>
        </NuxtLink>
      </div>
      <p v-else class="text-sm text-slate-500">暂无热门文章。</p>
    </section>

    <!-- featured projects -->
    <section class="space-y-4">
      <div class="flex items-baseline justify-between">
        <h2 class="text-xl font-bold tracking-tight">精选项目</h2>
        <NuxtLink to="/projects" class="text-sm text-blue-600 hover:underline dark:text-blue-400">全部项目 →</NuxtLink>
      </div>
      <div class="grid gap-4 sm:grid-cols-2">
        <NuxtLink
          v-for="p in featuredProjects"
          :key="p.slug"
          :to="`/projects/${p.slug}`"
          class="rounded-lg border border-slate-200 p-5 transition-shadow hover:shadow-md dark:border-slate-800"
        >
          <div class="flex items-center justify-between">
            <span class="text-2xl">{{ p.emoji }}</span>
            <span class="text-xs text-slate-500">{{ p.year }}</span>
          </div>
          <h3 class="mt-3 font-semibold">{{ p.name }}</h3>
          <p class="mt-1 text-sm text-slate-600 dark:text-slate-400">{{ p.summary }}</p>
          <div class="mt-3 flex flex-wrap gap-1.5">
            <span
              v-for="t in p.stack.slice(0, 4)"
              :key="t"
              class="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-400"
            >{{ t }}</span>
          </div>
        </NuxtLink>
      </div>
    </section>
  </div>
</template>
