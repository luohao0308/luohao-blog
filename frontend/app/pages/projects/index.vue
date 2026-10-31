<script setup lang="ts">
const projectsRoot = ref<HTMLElement | null>(null)
useArticleReveal(projectsRoot)
const selectedStack = ref('全部')
const query = ref('')
const { data, status, error, refresh } = await usePublicProjects()
const projects = computed(() => data.value ?? [])
const stacks = computed(() => ['全部', ...new Set(projects.value.flatMap(project => project.stack ?? []))])
const visibleProjects = computed(() => projects.value.filter(project => {
  const text = `${project.name} ${project.summary ?? ''} ${project.description ?? ''} ${(project.stack ?? []).join(' ')} ${project.role ?? ''}`.toLocaleLowerCase()
  return (selectedStack.value === '全部' || (project.stack ?? []).includes(selectedStack.value))
    && text.includes(query.value.trim().toLocaleLowerCase())
}))

usePageSeo({ title: '作品集', description: '查看 luohao 已公开的全栈工具、工程治理与 AI 协作项目。' })
</script>

<template>
  <div ref="projectsRoot" class="space-y-6">
    <InlineSearch v-model="query" label="搜索作品集">
      <template #actions>
        <NuxtLink to="/collections" class="inline-flex min-h-11 items-center gap-1.5 whitespace-nowrap rounded-full px-3 text-sm text-[var(--text-muted)] transition-colors hover:bg-[var(--surface-hover)] hover:text-[var(--accent)]">
          <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" class="size-4">
            <path d="M6 4.5h12v16l-6-4-6 4z" stroke-linejoin="round" />
          </svg>
          我的收藏
        </NuxtLink>
      </template>
      <template #heading><h1 class="shrink-0 text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">作品集</h1></template>
      <template v-if="stacks.length > 1" #filters>
        <nav aria-label="项目技术栈筛选" class="flex flex-wrap gap-2">
          <button
            v-for="stack in stacks"
            :key="stack"
            type="button"
            :aria-pressed="selectedStack === stack"
            class="max-w-full break-words rounded-full px-3.5 py-1.5 text-xs transition-colors"
            :class="selectedStack === stack ? 'bg-[#3c5d85] text-white dark:bg-blue-300 dark:text-slate-950' : 'bg-slate-100 text-slate-600 hover:bg-[#e5def1] dark:bg-slate-900 dark:text-slate-300 dark:hover:bg-slate-800'"
            @click="selectedStack = stack"
          >
            {{ stack }}
          </button>
        </nav>
      </template>
    </InlineSearch>
    <div v-if="query.trim() || selectedStack !== '全部'" class="flex flex-wrap gap-2 text-xs" aria-label="当前筛选">
      <button v-if="query.trim()" type="button" class="max-w-full break-words rounded-full bg-slate-100 px-3 py-1.5 text-slate-600 dark:bg-slate-800 dark:text-slate-300" :aria-label="`清除搜索：${query.trim()}`" @click="query = ''">搜索：{{ query.trim() }} ×</button>
      <button v-if="selectedStack !== '全部'" type="button" class="max-w-full break-words rounded-full bg-slate-100 px-3 py-1.5 text-slate-600 dark:bg-slate-800 dark:text-slate-300" :aria-label="`清除技术栈：${selectedStack}`" @click="selectedStack = '全部'">技术栈：{{ selectedStack }} ×</button>
    </div>
    <p v-if="!error && status !== 'pending'" role="status" class="text-xs text-slate-500 dark:text-slate-400">{{ visibleProjects.length }} 个项目</p>

    <div v-if="error" role="alert" class="space-y-3 py-16 text-center">
      <p class="text-sm text-slate-600 dark:text-slate-400">作品集暂时无法加载。</p>
      <button type="button" class="text-sm text-[#3c5d85] underline dark:text-blue-300" @click="refresh()">重新加载</button>
    </div>
    <p v-else-if="status === 'pending'" role="status" class="py-16 text-center text-sm text-slate-500">正在加载作品集…</p>
    <div v-else-if="visibleProjects.length" class="media-feed">
      <ProjectCard v-for="project in visibleProjects" :key="project.slug" :project="project" data-article-reveal />
    </div>
    <p v-else class="py-16 text-center text-sm text-slate-500 dark:text-slate-400">{{ projects.length ? '暂无匹配项目。' : '作品资料正在整理中。' }}</p>
  </div>
</template>
