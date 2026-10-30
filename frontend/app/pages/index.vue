<script setup lang="ts">
import { computed } from 'vue'
import AboutAircraftOverlay from '~/components/AboutAircraftOverlay.vue'
import { formatDate, usePublishedArticles } from '~/composables/useArticles'


const homeRoot = ref<HTMLElement | null>(null)
const { playing, skip } = useHomeEntrance(homeRoot)


usePageSeo({ title: '首页', description: '全栈工程师 luohao 的技术博客，记录 Python 后端、全栈开发、AIGC 与 AI Agent 的工程实践。' })

const socials = [
  { label: 'GitHub', href: 'https://github.com/luohao0308' },
  { label: 'Email', href: 'mailto:2429260713@qq.com' },
]

const { data: allArticles, status: articleStatus, error: articlesError, refresh: refreshArticles } = await usePublishedArticles()
const homeArticles = computed(() => (allArticles.value ?? []).slice(0, 3))
const { data: allProjects, error: projectsError, refresh: refreshProjects } = await usePublicProjects()

const stats = computed(() => {
  const articles = allArticles.value ?? []
  const tags = new Set(articles.flatMap(a => a.tags ?? []))
  return {
    articleCount: articles.length,
    projectCount: (allProjects.value ?? []).length,
    tagCount: tags.size,
    latestUpdated: articles[0]?.updated_at,
  }
})

const featuredProjects = computed(() => {
  const projects = allProjects.value ?? []
  const featured = projects.filter(project => project.featured)
  return (featured.length ? featured : projects).slice(0, 4)
})
const homeProjects = computed(() => featuredProjects.value.slice(0, 2))
const otherProjects = computed(() => featuredProjects.value.slice(2))
</script>

<template>
  <div ref="homeRoot" class="home-page" data-home-root :data-home-motion="playing ? 'playing' : 'idle'">
    <ClientOnly><AboutAircraftOverlay :playing="true" :reset-token="0" :show-controls="false" /></ClientOnly>
    <section class="home-hero" data-home-layer data-home-hero aria-labelledby="home-heading">
      <div class="home-intro">
        <p class="home-eyebrow"><span aria-hidden="true" />CODE · CREATE · EXPLORE</p>
        <h1 id="home-heading">把想法写成代码，<br>让探索<span>继续发生。</span></h1>
        <p class="home-description">你好，我是 luohao。分享做产品、写代码与探索 AI 的过程，<br class="hidden sm:block">记录真实的构建、思考，以及每一次向前。</p>
        <div class="home-actions">
          <NuxtLink to="/posts" class="ui-button home-primary">阅读文章 <span aria-hidden="true">↗</span></NuxtLink>
          <NuxtLink to="/projects" class="home-secondary">看看项目 <span aria-hidden="true">↗</span></NuxtLink>
        </div>
        <nav aria-label="作者联系方式" class="home-socials">
          <a v-for="social in socials" :key="social.label" :href="social.href" target="_blank" rel="noopener noreferrer">{{ social.label }} <span aria-hidden="true">↗</span></a>
        </nav>
      </div>
      <HomeVisual :playing="true" :reset-token="0" />
    </section>

    <div class="home-stats" data-home-layer="subtle" aria-label="站点概况">
      <span><strong>{{ stats.articleCount }}</strong> 篇文章</span>
      <span><strong>{{ stats.projectCount }}</strong> 个项目</span>
      <span><strong>{{ stats.tagCount }}</strong> 个标签</span>
      <time v-if="formatDate(stats.latestUpdated)">更新于 {{ formatDate(stats.latestUpdated) }}</time>
    </div>

    <section class="home-section" data-home-layer="subtle" aria-label="主推项目">
      <div class="home-section-heading">
        <div><h2>精选项目</h2></div>
        <NuxtLink to="/projects" class="home-text-link">全部项目 <span aria-hidden="true">↗</span></NuxtLink>
      </div>
      <div v-if="homeProjects.length" class="home-projects"><div v-for="project in homeProjects" :key="project.slug" class="home-card-shell" data-home-card><ProjectCard :project="project" /></div></div>
      <div v-if="otherProjects.length" class="home-project-links">
        <NuxtLink v-for="project in otherProjects" :key="project.slug" :to="`/projects/${encodeURIComponent(project.slug)}`">
          <span>{{ project.name }} <span aria-hidden="true">↗</span></span>
          <p>{{ project.summary }}</p>
        </NuxtLink>
      </div>
      <div v-if="projectsError" role="alert" class="home-empty">精选项目暂时无法加载。<button type="button" class="ml-2 underline" @click="refreshProjects()">重新加载</button></div>
      <p v-else-if="!homeProjects.length" class="home-empty">项目资料正在整理中。</p>
    </section>

    <section class="home-section" data-home-layer aria-label="最新文章">
      <div class="home-section-heading">
        <div><h2>最新文章</h2></div>
        <NuxtLink to="/posts" class="home-text-link">全部文章 <span aria-hidden="true">↗</span></NuxtLink>
      </div>
      <div v-if="articlesError" role="alert" class="home-empty">文章暂时加载失败。<button type="button" class="ml-2 underline" @click="refreshArticles()">重新加载</button></div>
      <p v-else-if="articleStatus === 'pending'" role="status" class="home-empty">正在加载文章…</p>
      <div v-else-if="homeArticles.length" class="home-articles">
        <div v-for="article in homeArticles" :key="article.id" class="home-card-shell" data-home-card><ArticleCard :article="article" /></div>
      </div>
      <p v-else class="home-empty">第一篇文章正在路上。</p>
    </section>

    <Teleport to="body">
      <button v-if="playing" data-home-skip type="button" class="home-skip" @click="skip">跳过开场 <span aria-hidden="true">→</span></button>
    </Teleport>
  </div>
</template>

<style scoped>
.home-page :deep(.home-visual) { min-height: 230px; max-width: 310px; }
.home-page { max-width: 840px; margin-inline: auto; perspective: 1600px; }
.home-hero { display: grid; grid-template-columns: minmax(0, 1.05fr) minmax(0, .95fr); align-items: center; gap: 2rem; padding: 1.8rem 0 1.6rem; }
.home-intro { min-width: 0; transform-style: preserve-3d; }
.home-eyebrow { display: flex; align-items: center; gap: .6rem; font: 500 .6875rem/1.5 ui-monospace, monospace; letter-spacing: .14em; color: var(--text-muted); }
.home-eyebrow > span { width: 6px; height: 6px; border-radius: 50%; background: var(--mint-text); }
.home-intro h1 { margin: 1rem 0; font-size: clamp(1.9rem, 3vw, 2.6rem); line-height: 1.4; font-weight: 600; letter-spacing: -.055em; color: var(--text-primary); }
.home-intro h1 span { color: var(--accent); }
.home-description { color: var(--text-muted); font-size: .8125rem; line-height: 2; }
.home-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 1.4rem; margin-top: 1.2rem; }
.home-primary { gap: 1.3rem; padding: .5rem 1rem; font-size: .75rem; }
.home-secondary, .home-socials a, .home-text-link { display: inline-flex; align-items: center; gap: .6rem; min-height: 44px; font-size: .75rem; }
.home-secondary, .home-text-link { color: var(--accent); }
.home-socials { display: flex; gap: 1.3rem; margin-top: .6rem; color: var(--text-muted); }
.home-socials a { font-size: .75rem; }
.home-socials a:hover, .home-secondary:hover, .home-text-link:hover { color: var(--accent-hover); }
.home-stats { display: flex; flex-wrap: wrap; gap: .7rem 1.8rem; align-items: center; padding: .85rem 0; border-top: 1px solid var(--border-default); border-bottom: 1px solid var(--border-default); font-size: .6875rem; color: var(--text-muted); font-variant-numeric: tabular-nums; }
.home-stats strong { color: var(--text-primary); font-weight: 550; }
.home-stats time { margin-left: auto; }
.home-section { padding-top: 2rem; }
.home-section-heading { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin-bottom: .85rem; }

.home-section-heading h2 { font-size: 1rem; font-weight: 550; letter-spacing: -.035em; }

.home-card-shell { display: flex; min-width: 0; border-radius: var(--radius-panel); transform-origin: 50% 70%; }


.home-card-shell > :deep(.editorial-card) { flex: 1; }
.home-projects { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }

.home-project-links { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1.6rem; margin-top: .4rem; }
.home-project-links > a { min-width: 0; padding: 1rem 0; border-bottom: 1px solid var(--border-default); }
.home-project-links > a > span { display: flex; align-items: center; justify-content: space-between; gap: 1rem; font-size: .8125rem; font-weight: 550; }
.home-project-links p { margin-top: .3rem; font-size: .6875rem; line-height: 1.9; color: var(--text-muted); }
.home-articles { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.home-project-links a:hover { color: var(--accent); }

.home-empty { padding: 2.5rem 0; color: var(--text-muted); font-size: .875rem; }
.home-skip { position: fixed; right: 1.4rem; bottom: 1.4rem; z-index: 70; display: inline-flex; align-items: center; gap: 1.2rem; min-height: 44px; padding: .6rem 1rem; border: 1px solid var(--border-strong); border-radius: var(--radius-control); background: var(--surface-overlay); color: var(--text-primary); box-shadow: 0 8px 25px rgb(31 55 92 / 12%); font-size: .8125rem; }
@media (max-width: 900px) { .home-hero { gap: 1.2rem; } .home-articles { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 640px) {
  .home-projects { grid-template-columns: minmax(0, 1fr); }
  .home-hero { grid-template-columns: minmax(0, 1fr); padding: 1.1rem 0 1.5rem; gap: 1rem; }
  .home-intro h1 { font-size: clamp(1.8rem, 7.5vw, 2.2rem); margin: 1rem 0; }
  .home-description { font-size: .8125rem; }
  .home-stats { gap: .7rem 1.1rem; }
  .home-stats time { margin-left: 0; width: 100%; }
  .home-section { padding-top: 2rem; }
  .home-section-heading h2 { font-size: 1rem; }
  .home-project-links { grid-template-columns: minmax(0, 1fr); gap: 0; }
  .home-articles { grid-template-columns: minmax(0, 1fr); }
  .home-skip { right: .8rem; bottom: .8rem; }
}
</style>
