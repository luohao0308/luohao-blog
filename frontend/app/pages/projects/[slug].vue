<script setup lang="ts">
import { projectImages } from '~/data/editorial'
const route = useRoute()
definePageMeta({ key: route => route.path })
const slug = computed(() => String(route.params.slug))
const { data, error } = await useProjectBySlug(slug.value)

if (error.value) throw createError({ statusCode: error.value.statusCode === 404 ? 404 : 503, message: error.value.statusCode === 404 ? '项目不存在' : '项目暂时无法加载' })
if (!data.value) throw createError({ statusCode: 404, message: '项目不存在' })
const project = computed(() => data.value!)
const editorialCover = computed(() => project.value.cover_url ? undefined : projectImages[slug.value])
const sections = computed(() => {
  const source = project.value.description
  if (!source.startsWith('## ')) return [{ title: '项目说明', paragraphs: source.split('\n\n') }]
  return source.split(/\n\n(?=## )/).map(section => {
    const [title, ...body] = section.split('\n')
    return { title: title!.replace(/^## /, ''), paragraphs: body.join('\n').trim().split('\n\n') }
  })
})
const repoURL = computed(() => safePortfolioHref(project.value.repo_url))
const liveURL = computed(() => safePortfolioHref(project.value.live_url))
const hasCaseDetails = computed(() => Boolean(project.value.problem || project.value.approach || project.value.outcome))

usePageSeo({
  title: () => `${project.value.name} · 作品集`,
  description: () => project.value.summary,
  image: () => project.value.cover_url || editorialCover.value?.seoSrc,
})
</script>

<template>
  <article v-if="project" data-project-detail class="mx-auto w-full max-w-[860px] space-y-10">
    <header class="space-y-5 border-b border-slate-200 pb-9 dark:border-slate-800">
      <NuxtLink to="/projects" class="detail-button">← 返回作品集</NuxtLink>
      <div class="flex flex-wrap items-start justify-between gap-5">
        <div class="flex min-w-0 items-start gap-4">
          <span class="flex h-14 w-14 shrink-0 items-center justify-center rounded-xl bg-[#e8eef7] text-3xl dark:bg-slate-900" aria-hidden="true">{{ project.emoji }}</span>
          <div class="min-w-0">
            <p class="text-xs text-[#3c5d85] dark:text-blue-300">{{ project.role }} · {{ project.year }}</p>
            <h1 tabindex="-1" class="mt-2 break-words text-3xl font-medium text-slate-800 dark:text-slate-100">{{ project.name }}</h1>
          </div>
        </div>

      </div>
      <p class="max-w-2xl text-base leading-8 text-slate-600 dark:text-slate-400">{{ project.summary }}</p>
    </header>

    <div class="grid gap-10 lg:grid-cols-[minmax(0,1fr)_220px]">
      <section class="min-w-0 space-y-4">
        <section v-for="section in sections" :key="section.title" class="space-y-4 pb-5">
          <h2 class="text-lg font-medium text-slate-800 dark:text-slate-100">{{ section.title }}</h2>
          <p v-for="paragraph in section.paragraphs" :key="paragraph" class="max-w-[860px] whitespace-pre-line break-words text-sm leading-8 text-slate-600 dark:text-slate-400">{{ paragraph }}</p>
        </section>
      </section>
      <aside class="bento-card self-start space-y-4 rounded-xl bg-[#f3f5f8] p-5 dark:bg-slate-900">
        <h2 class="text-xs font-medium text-slate-700 dark:text-slate-200">技术栈</h2>
        <ul class="space-y-2 text-xs text-slate-600 dark:text-slate-400">
          <li v-for="item in project.stack" :key="item" class="break-words">{{ item }}</li>
        </ul>
      </aside>
    </div>

    <section v-if="hasCaseDetails" class="grid gap-5 sm:grid-cols-3">
      <div v-for="item in [{ title: '问题', text: project.problem }, { title: '方案', text: project.approach }, { title: '结果', text: project.outcome }]" :key="item.title" class="bento-card rounded-xl border border-slate-200 p-5 dark:border-slate-800">
        <h2 class="text-sm font-medium text-slate-800 dark:text-slate-100">{{ item.title }}</h2>
        <p class="mt-3 max-w-[860px] whitespace-pre-line break-words text-sm leading-7 text-slate-600 dark:text-slate-400">{{ item.text || '资料正在整理中。' }}</p>
      </div>
    </section>

    <section v-if="project.screenshots?.length" class="space-y-4">
      <h2 class="text-lg font-medium text-slate-800 dark:text-slate-100">项目截图</h2>
      <div class="grid gap-4 sm:grid-cols-2">
        <img v-for="(image, index) in project.screenshots" :key="image" :src="assetUrl(image)" :alt="`${project.name} 截图 ${index + 1}`" class="detail-media rounded-xl border border-[var(--border-default)]">
      </div>
    </section>

    <footer class="flex flex-wrap items-center justify-between gap-4 border-t border-[var(--border-default)] pt-8">
      <NuxtLink to="/projects" class="detail-button">← 返回作品集</NuxtLink>
      <div v-if="repoURL || liveURL" class="flex flex-wrap gap-3">
        <a v-if="repoURL" :href="repoURL" target="_blank" rel="noopener" class="detail-button">查看源码 ↗</a>
        <a v-if="liveURL" :href="liveURL" target="_blank" rel="noopener" class="detail-button">在线体验 ↗</a>
      </div>
    </footer>
  </article>
</template>
