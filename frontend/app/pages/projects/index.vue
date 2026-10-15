<script setup lang="ts">
const selectedStack = ref('全部')
const { data, status, error, refresh } = await usePublicProjects()
const projects = computed(() => data.value ?? [])
const stacks = computed(() => ['全部', ...new Set(projects.value.flatMap(project => project.stack ?? []))])
const visibleProjects = computed(() => selectedStack.value === '全部'
  ? projects.value
  : projects.value.filter(project => (project.stack ?? []).includes(selectedStack.value)))

usePageSeo({ title: '作品集', description: '查看 luohao 已公开的全栈工具、工程治理与 AI 协作项目。' })
</script>

<template>
  <div class="space-y-8">
    <PageHeader
      title="作品集"
      :description="`把经常遇到的问题做成工具，也把做工具时的判断记录下来。共 ${projects.length} 个项目、${stacks.length - 1} 个技术方向。`"
    />

    <nav v-if="projects.length" aria-label="项目技术栈筛选" class="flex gap-2 overflow-x-auto pb-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
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

    <div v-if="error" role="alert" class="space-y-3 py-16 text-center">
      <p class="text-sm text-slate-600 dark:text-slate-400">作品集暂时无法加载。</p>
      <button type="button" class="text-sm text-[#3c5d85] underline dark:text-blue-300" @click="refresh()">重新加载</button>
    </div>
    <p v-else-if="status === 'pending'" role="status" class="py-16 text-center text-sm text-slate-500">正在加载作品集…</p>
    <div v-else-if="visibleProjects.length" class="grid gap-6 sm:grid-cols-2">
      <ProjectCard v-for="project in visibleProjects" :key="project.slug" :project="project" />
    </div>
    <p v-else class="py-16 text-center text-sm text-slate-500 dark:text-slate-400">{{ projects.length ? '暂无匹配项目。' : '作品资料正在整理中。' }}</p>
  </div>
</template>
