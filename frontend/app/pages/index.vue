<script setup lang="ts">
import { projects } from '~/data/projects'
import { formatDate, useArticleList } from '~/composables/useArticles'

const { data } = useArticleList({ pageSize: 3 })

useHead({ title: '首页' })

const socials = [
  { label: 'GitHub', href: 'https://github.com/luohao0308', text: 'GitHub' },
  { label: '邮箱', href: 'mailto:hello@example.com', text: 'Email' },
]

const featuredProjects = projects.filter((p) => p.featured)
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
            <span>{{ a.tags[0] ?? '随笔' }}</span>
          </div>
          <h3 class="mt-2 font-semibold">{{ a.title }}</h3>
          <p class="mt-1 line-clamp-2 text-sm text-slate-600 dark:text-slate-400">
            {{ a.summary || a.content_md?.slice(0, 60) }}
          </p>
        </NuxtLink>
      </div>
      <p v-else class="text-sm text-slate-500">第一篇文章正在路上。</p>
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
