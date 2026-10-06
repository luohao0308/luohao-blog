<script setup lang="ts">
import { projects } from '~/data/projects'

const selectedStack = ref('全部')
const stacks = computed(() => ['全部', ...new Set(projects.flatMap(project => project.stack))])
const visibleProjects = computed(() => selectedStack.value === '全部'
  ? projects
  : projects.filter(project => project.stack.includes(selectedStack.value)))

useHead({ title: '作品集' })
</script>

<template>
  <div class="space-y-8">
    <PageHeader
      title="作品集"
      :description="`把经常遇到的问题做成工具，也把做工具时的判断记录下来。共 ${projects.length} 个项目、${stacks.length - 1} 个技术方向。`"
    />

    <nav aria-label="项目技术栈筛选" class="flex gap-2 overflow-x-auto pb-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
      <button
        v-for="stack in stacks"
        :key="stack"
        type="button"
        :aria-pressed="selectedStack === stack"
        class="shrink-0 rounded-full px-3.5 py-1.5 text-xs transition-colors"
        :class="selectedStack === stack ? 'bg-[#3c5d85] text-white dark:bg-blue-300 dark:text-slate-950' : 'bg-slate-100 text-slate-600 hover:bg-[#e5def1] dark:bg-slate-900 dark:text-slate-300 dark:hover:bg-slate-800'"
        @click="selectedStack = stack"
      >
        {{ stack }}
      </button>
    </nav>

    <div v-if="visibleProjects.length" class="grid gap-6 sm:grid-cols-2">
      <ProjectCard v-for="project in visibleProjects" :key="project.slug" :project="project" />
    </div>
    <p v-else class="py-16 text-center text-sm text-slate-500 dark:text-slate-400">暂无匹配项目。</p>
  </div>
</template>
