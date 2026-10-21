<script setup lang="ts">
import { projects as seedProjects } from '~/data/projects'
import type { Project } from '~/composables/usePortfolio'
import type { Article } from '~/composables/useArticles'

if (!import.meta.dev) throw createError({ statusCode: 404, statusMessage: 'Page not found' })
definePageMeta({ layout: false })
useHead({ title: '封面式卡片 · 交互预览', meta: [{ name: 'robots', content: 'noindex, nofollow' }] })
const cardRoot = ref<HTMLElement | null>(null)
useCardSpotlight(cardRoot)
const projects: Project[] = seedProjects.map((project, i) => ({ ...project, published: true, sort: i }))
const { init, toggle } = useTheme()
onMounted(init)
const article: Article = {
  id: 'preview', slug: 'preview-article', title: '用 Go 和 Nuxt，构建自己的数字花园',
  summary: '从一个想法到一个可持续维护的个人站，记录设计、开发与迭代中的取舍。',
  tags: ['Go', 'Nuxt'], status: 2, created_at: '2026-10-21T00:00:00Z', updated_at: '2026-10-21T00:00:00Z', content_md: '', content_html: '',
}
const agentArticle: Article = { ...article, id: 'agent', slug: 'preview-agent', title: '把 AI Agent 接入真实开发流程', summary: '把上下文管理、交付门禁与回归验证，变成可以执行的工程约定。', tags: ['AI Agent', '工程实践'] }
const desk = 'https://images.unsplash.com/photo-1499750310107-5fef28a66643?auto=format&fit=crop&w=800&q=80'
const laptop = 'https://images.unsplash.com/photo-1496181133206-80ce9b88a853?auto=format&fit=crop&w=800&q=80'
</script>

<template>
  <main ref="cardRoot" class="site-shell min-h-screen bg-[#fafaf9] px-5 py-8 text-slate-900 dark:bg-[#0b111a] dark:text-slate-100">
    <div class="mx-auto max-w-[920px]">
      <header class="mb-8 flex items-start justify-between gap-5">
        <div>
          <p class="mb-2 text-xs text-[#566b91] dark:text-blue-300">封面式卡片</p>
          <h1 class="text-2xl font-semibold tracking-tight">封面与交互，都保留。</h1>
          <p class="mt-3 text-xs leading-6 text-slate-500 dark:text-slate-400">有图、无图等高混排。悬停查看浮起、封面放大和箭头反馈，也支持键盘焦点。</p>
        </div>
        <button type="button" class="shrink-0 rounded-full border border-slate-200 px-3 py-2 text-xs dark:border-slate-700" @click="toggle">切换明暗</button>
      </header>
      <section class="mb-7">
        <h2 class="mb-4 text-sm font-medium">文章</h2>
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <ArticleCard :article="article" :cover-src="desk" cover-alt="工作桌面，示例照片" />
          <ArticleCard :article="article" />
          <ArticleCard :article="agentArticle" :cover-src="laptop" cover-alt="笔记本电脑，示例照片" />
        </div>
      </section>
      <section>
        <h2 class="mb-4 text-sm font-medium">作品集</h2>
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <ProjectCard v-for="(project, i) in projects" :key="project.slug" :project="project" :cover-src="i === 1 ? laptop : undefined" cover-alt="笔记本电脑，示例照片" />
        </div>
      </section>
      <p class="mt-6 text-[11px] text-slate-400">照片来自 Unsplash，仅演示模板，正式内容稍后补充。此页面仅在本地开发环境开放。</p>
    </div>
  </main>
</template>
