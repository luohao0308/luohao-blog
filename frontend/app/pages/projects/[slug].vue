<script setup lang="ts">
import { getProject, projects } from '~/data/projects'

const route = useRoute()
const slug = computed(() => String(route.params.slug))
const project = computed(() => getProject(slug.value))

if (!project.value) {
  throw createError({ statusCode: 404, message: '项目不存在' })
}

// Related projects rank by shared stack entries. Zero-overlap projects still
// show (ranked last): the portfolio is small, and the section doubles as
// "what else is here" navigation rather than disappearing entirely.
const relatedProjects = computed(() =>
  projects
    .filter(item => item.slug !== project.value?.slug)
    .map(item => ({ project: item, overlap: item.stack.filter(s => project.value?.stack.includes(s)).length }))
    .sort((a, b) => b.overlap - a.overlap)
    .slice(0, 2)
    .map(entry => entry.project),
)

useHead(() => ({ title: `${project.value?.name ?? '项目'} · 作品集` }))
</script>

<template>
  <article v-if="project" class="space-y-10">
    <header class="space-y-5 border-b border-slate-200 pb-9 dark:border-slate-800">
      <NuxtLink to="/projects" class="text-xs text-[#3c5d85] hover:underline dark:text-blue-300">返回作品集</NuxtLink>
      <div class="flex flex-wrap items-start justify-between gap-5">
        <div class="flex items-start gap-4">
          <span class="flex h-14 w-14 items-center justify-center rounded-lg bg-[#e8eef7] text-3xl dark:bg-slate-900" aria-hidden="true">{{ project.emoji }}</span>
          <div>
            <p class="text-xs text-[#3c5d85] dark:text-blue-300">{{ project.role }} · {{ project.year }}</p>
            <h1 class="mt-2 break-words text-3xl font-medium text-slate-800 dark:text-slate-100">{{ project.name }}</h1>
          </div>
        </div>
        <div class="flex flex-wrap gap-2">
          <a v-if="project.repoUrl" :href="project.repoUrl" target="_blank" rel="noopener" class="rounded-md border border-slate-300 px-3 py-2 text-xs text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-900">查看源码 ↗</a>
          <a v-if="project.liveUrl" :href="project.liveUrl" target="_blank" rel="noopener" class="rounded-md bg-[#3c5d85] px-3 py-2 text-xs text-white hover:bg-[#2d486b] dark:bg-blue-300 dark:text-slate-950">在线体验 ↗</a>
        </div>
      </div>
      <p class="max-w-2xl text-base leading-8 text-slate-600 dark:text-slate-400">{{ project.summary }}</p>
    </header>

    <div class="grid gap-10 lg:grid-cols-[1fr_220px]">
      <section class="space-y-4">
        <h2 class="text-lg font-medium text-slate-800 dark:text-slate-100">项目说明</h2>
        <p class="whitespace-pre-line text-sm leading-8 text-slate-600 dark:text-slate-400">{{ project.description }}</p>
      </section>
      <aside class="space-y-4 rounded-lg bg-[#f3f5f8] p-5 dark:bg-slate-900">
        <h2 class="text-xs font-medium text-slate-700 dark:text-slate-200">技术栈</h2>
        <ul class="space-y-2 text-xs text-slate-600 dark:text-slate-400">
          <li v-for="item in project.stack" :key="item" class="break-words">{{ item }}</li>
        </ul>
      </aside>
    </div>

    <section v-if="relatedProjects.length" class="space-y-4 border-t border-slate-200 pt-9 dark:border-slate-800">
      <h2 class="text-lg font-medium text-slate-800 dark:text-slate-100">相关项目</h2>
      <div class="grid gap-6 sm:grid-cols-2">
        <ProjectCard v-for="item in relatedProjects" :key="item.slug" :project="item" />
      </div>
    </section>

    <nav aria-label="项目导航" class="border-t border-slate-200 pt-6 dark:border-slate-800">
      <NuxtLink to="/projects" class="text-sm text-[#3c5d85] hover:underline dark:text-blue-300">浏览全部项目 →</NuxtLink>
    </nav>
  </article>
</template>
