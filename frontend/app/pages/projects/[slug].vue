<script setup lang="ts">
import { projectImages, projectStories } from '~/data/editorial'
import { projectArticleSlugs } from '~/data/projectArticles'
const route = useRoute()
definePageMeta({ key: route => route.path })
const slug = computed(() => String(route.params.slug))
const { data, error } = await useProjectBySlug(slug.value)

if (error.value) throw createError({ statusCode: error.value.statusCode === 404 ? 404 : 503, message: error.value.statusCode === 404 ? '项目不存在' : '项目暂时无法加载' })
if (!data.value) throw createError({ statusCode: 404, message: '项目不存在' })
const project = computed(() => data.value!)
const editorialCover = computed(() => project.value.cover_url ? undefined : projectImages[slug.value])
const story = computed(() => projectStories[slug.value])
const sections = computed(() => {
  const source = project.value.description
  if (!source.startsWith('## ')) return [{ title: '项目说明', paragraphs: source.split('\n\n') }]
  return source.split(/\n\n(?=## )/).map(section => {
    const [title, ...body] = section.split('\n')
    return { title: title!.replace(/^## /, ''), paragraphs: body.join('\n').trim().split('\n\n') }
  })
})
const { data: allProjects, error: relatedError } = await usePublicProjects()
const { data: published, error: articleError, status: articleStatus, refresh: refreshArticles } = await usePublishedArticles()
const hasCuratedArticles = computed(() => !!projectArticleSlugs[slug.value]?.length)
const practiceArticles = computed(() => (published.value ?? []).filter(article => projectArticleSlugs[slug.value]?.includes(article.slug)))
const repoURL = computed(() => safePortfolioHref(project.value.repo_url))
const liveURL = computed(() => safePortfolioHref(project.value.live_url))
const hasCaseDetails = computed(() => Boolean(project.value.problem || project.value.approach || project.value.outcome))

// Related projects rank by shared stack entries. Zero-overlap projects still
// show (ranked last): the portfolio is small, and the section doubles as
// "what else is here" navigation rather than disappearing entirely.
const relatedProjects = computed(() =>
  (allProjects.value ?? [])
    .filter(item => item.slug !== project.value?.slug)
    .map(item => ({ project: item, overlap: item.stack.filter(s => project.value?.stack.includes(s)).length }))
    .sort((a, b) => b.overlap - a.overlap)
    .slice(0, 2)
    .map(entry => entry.project),
)

usePageSeo({
  title: () => `${project.value.name} · 作品集`,
  description: () => project.value.summary,
  image: () => project.value.cover_url || editorialCover.value?.seoSrc,
})
</script>

<template>
  <article v-if="project" data-project-detail class="mx-auto w-full max-w-[860px] space-y-10">
    <header class="space-y-5 border-b border-slate-200 pb-9 dark:border-slate-800">
      <NuxtLink to="/projects" class="text-xs text-[#3c5d85] hover:underline dark:text-blue-300">返回作品集</NuxtLink>
      <div class="flex flex-wrap items-start justify-between gap-5">
        <div class="flex min-w-0 items-start gap-4">
          <span class="flex h-14 w-14 shrink-0 items-center justify-center rounded-xl bg-[#e8eef7] text-3xl dark:bg-slate-900" aria-hidden="true">{{ project.emoji }}</span>
          <div class="min-w-0">
            <p class="text-xs text-[#3c5d85] dark:text-blue-300">{{ project.role }} · {{ project.year }}</p>
            <h1 tabindex="-1" class="mt-2 break-words text-3xl font-medium text-slate-800 dark:text-slate-100">{{ project.name }}</h1>
          </div>
        </div>
        <div class="flex flex-wrap gap-2">
          <a v-if="repoURL" :href="repoURL" target="_blank" rel="noopener" class="rounded-xl border border-slate-300 px-3 py-2 text-xs text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-900">查看源码 ↗</a>
          <a v-if="liveURL" :href="liveURL" target="_blank" rel="noopener" class="rounded-full bg-[#3c5d85] px-3 py-2 text-xs text-white hover:bg-[#2d486b] dark:bg-blue-300 dark:text-slate-950">在线体验 ↗</a>
        </div>
      </div>
      <p class="max-w-2xl text-base leading-8 text-slate-600 dark:text-slate-400">{{ project.summary }}</p>
      <ContentFigure v-if="editorialCover" :media="editorialCover" />
      <img v-else-if="project.cover_url" :src="assetUrl(project.cover_url)" :alt="`${project.name} 项目封面`" class="aspect-[16/9] w-full rounded-xl border border-slate-200 object-cover dark:border-slate-800">
    </header>

    <div class="grid gap-10 lg:grid-cols-[minmax(0,1fr)_220px]">
      <section class="min-w-0 space-y-4">
        <section v-for="(section, index) in sections" :key="section.title" class="space-y-4 pb-5">
          <h2 class="text-lg font-medium text-slate-800 dark:text-slate-100">{{ section.title }}</h2>
          <p v-for="paragraph in section.paragraphs" :key="paragraph" class="max-w-[860px] whitespace-pre-line break-words text-sm leading-8 text-slate-600 dark:text-slate-400">{{ paragraph }}</p>
          <ContentFigure v-if="index === 1 && story?.detailImage" :media="story.detailImage" />
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
    <p v-else-if="!story" class="rounded-xl border border-dashed border-slate-300 p-5 text-sm text-slate-500 dark:border-slate-700 dark:text-slate-400">问题、方案与结果资料正在整理中，当前仅展示已经核实的项目信息。</p>

    <section v-if="project.screenshots?.length" class="space-y-4">
      <h2 class="text-lg font-medium text-slate-800 dark:text-slate-100">项目截图</h2>
      <div class="grid gap-4 sm:grid-cols-2">
        <img v-for="(image, index) in project.screenshots" :key="image" :src="assetUrl(image)" :alt="`${project.name} 截图 ${index + 1}`" class="w-full rounded-xl border border-slate-200 object-cover dark:border-slate-800">
      </div>
    </section>

    <ContentFigure v-if="story" :media="{ src: `/images/editorial/${story.diagram}.svg`, alt: `${project.name} 的项目链路简图`, caption: '项目链路简图。', credit: '', diagram: true }" />
    <section v-if="story" class="space-y-4 border-t border-slate-200 pt-8 dark:border-slate-800">
      <h2 class="text-lg font-medium">源码与延伸阅读</h2>
      <ul class="space-y-3 text-sm leading-7">
        <li v-for="reference in story.references" :key="reference.url"><a :href="reference.url" target="_blank" rel="noopener" class="text-blue-600 underline dark:text-blue-300">{{ reference.title }} ↗</a></li>
      </ul>
    </section>

    <section v-if="practiceArticles.length || (hasCuratedArticles && (articleError || articleStatus === 'pending'))" aria-label="项目实践文章" class="border-t border-[var(--border-default)] pt-8">
      <h2 class="text-lg font-semibold">实践与技术文章</h2>
      <p class="mt-2 text-sm text-[var(--text-muted)]">继续阅读相关实现、问题排查与案例推演。</p>
      <div v-if="articleError" role="alert" class="mt-5 text-sm text-[var(--text-muted)]">相关文章暂时加载失败。<button type="button" class="ml-2 text-[var(--accent)] underline" @click="refreshArticles()">重新加载</button></div>
      <p v-else-if="articleStatus === 'pending'" role="status" class="mt-5 text-sm text-[var(--text-muted)]">正在加载相关文章…</p>
      <div v-else class="media-feed mt-5"><ArticleCard v-for="article in practiceArticles" :key="article.id" :article="article" /></div>
    </section>

    <section v-if="relatedProjects.length" class="space-y-4 border-t border-slate-200 pt-9 dark:border-slate-800">
      <h2 class="text-lg font-medium text-slate-800 dark:text-slate-100">相关项目</h2>
      <div class="grid gap-6 sm:grid-cols-2">
        <ProjectCard v-for="item in relatedProjects" :key="item.slug" :project="item" />
      </div>
    </section>
    <p v-if="relatedError" class="text-sm text-slate-500 dark:text-slate-400">相关项目暂时无法加载。</p>

    <nav aria-label="项目导航" class="border-t border-slate-200 pt-6 dark:border-slate-800">
      <NuxtLink to="/projects" class="text-sm text-[#3c5d85] hover:underline dark:text-blue-300">浏览全部项目 →</NuxtLink>
    </nav>
  </article>
</template>
