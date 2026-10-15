<script setup lang="ts">
import type { Project } from '~/composables/usePortfolio'

defineProps<{ project: Project }>()
</script>

<template>
  <article class="group flex h-full flex-col overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-950">
    <div class="relative flex min-h-40 items-center justify-between overflow-hidden bg-[#e8eef7] transition-colors group-hover:bg-[#e5def1] dark:bg-slate-900 dark:group-hover:bg-slate-800">
      <img v-if="project.cover_url" :src="assetUrl(project.cover_url)" :alt="`${project.name} 项目封面`" class="absolute inset-0 h-full w-full object-cover transition-transform duration-300 group-hover:scale-[1.02]">
      <span v-else class="px-6 text-5xl" aria-hidden="true">{{ project.emoji || '🧩' }}</span>
      <span class="relative ml-auto mr-4 mt-auto rounded-full bg-white/85 px-2 py-1 font-mono text-xs text-slate-600 backdrop-blur dark:bg-slate-950/80 dark:text-slate-300">{{ project.year }}</span>
    </div>
    <div class="flex flex-1 flex-col p-6">
      <div class="flex items-start justify-between gap-4">
        <div class="min-w-0">
          <p class="text-xs text-[#3c5d85] dark:text-blue-300">{{ project.role }}</p>
          <h2 class="mt-2 break-words text-xl font-medium text-slate-800 dark:text-slate-100">
            <NuxtLink :to="`/projects/${encodeURIComponent(project.slug)}`" class="hover:text-[#3c5d85] dark:hover:text-blue-300">
              {{ project.name }}
            </NuxtLink>
          </h2>
        </div>
        <span class="text-slate-400 transition-transform group-hover:translate-x-1" aria-hidden="true">↗</span>
      </div>
      <p class="mt-3 text-sm leading-7 text-slate-600 dark:text-slate-400">{{ project.summary }}</p>
      <p v-if="project.outcome" class="mt-3 line-clamp-2 text-xs leading-6 text-slate-500 dark:text-slate-400">{{ project.outcome }}</p>
      <div class="mt-auto flex flex-wrap gap-2 pt-6">
        <span v-for="item in project.stack" :key="item" class="max-w-full break-words rounded-sm bg-slate-100 px-2 py-1 text-xs text-slate-600 dark:bg-slate-900 dark:text-slate-400">{{ item }}</span>
      </div>
    </div>
  </article>
</template>
