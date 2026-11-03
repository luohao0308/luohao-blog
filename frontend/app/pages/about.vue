<script setup lang="ts">
import { usePublishedArticles } from '~/composables/useArticles'

definePageMeta({ immersiveAbout: true })

const { data: about, status, error, refresh } = await useAboutProfile()
const { data: projects, error: projectsError } = await usePublicProjects()
const { data: articles, error: articlesError } = await usePublishedArticles()

const representativeProjects = computed(() => {
  const list = projects.value ?? []
  const featured = list.filter(project => project.featured)
  return (featured.length ? featured : list).slice(0, 2)
})
const displayIntro = '主要做 Python 后端、全栈开发和 AI 应用。'
const representativeArticles = computed(() => (articles.value ?? []).slice(0, 2))
usePageSeo({
  title: () => about.value?.name ? `关于 ${about.value.name}` : '关于我',
  description: displayIntro,
  image: () => about.value?.profile_image,
})
</script>

<template>
  <div class="about-page">
    <div v-if="error" role="alert" class="space-y-3 py-20 text-center">
      <p class="text-sm text-slate-600 dark:text-slate-400">关于页面暂时无法加载。</p>
      <button type="button" class="text-sm text-[#3c5d85] underline dark:text-blue-300" @click="refresh()">重新加载</button>
    </div>
    <p v-else-if="status === 'pending'" role="status" class="py-20 text-center text-sm text-slate-500">正在加载关于页面…</p>
    <AboutScrollReveal v-else-if="about" class="about-focus-story">
      <AboutPointerBackground />
      <section data-section class="about-intro">
        <h1 data-reveal>你好，我是罗浩。</h1>
        <p data-reveal>{{ displayIntro }}</p>
        <img v-if="about.profile_image" data-reveal :src="assetUrl(about.profile_image)" alt="罗浩头像" width="120" height="150" class="about-portrait">
        <a data-reveal href="#about-writing" class="scroll-hint">向下滚动，继续了解 <span aria-hidden="true">↓</span></a>
      </section>
      <div data-about-content class="about-story">
        <section id="about-writing" data-section>
          <h2 data-reveal>文章</h2>
          <div class="reader-notes">
            <p data-reveal><strong>后端开发</strong>：Python、Go、数据库和接口。</p>
            <p data-reveal><strong>AI 应用</strong>：模型接入、Agent 和工作流。</p>
            <p data-reveal><strong>项目记录</strong>：开发中遇到的问题和解决办法。</p>
          </div>
        </section>

        <section data-section>
          <h2 data-reveal>项目</h2>
          <div class="reader-notes">
            <p data-reveal><strong>dev-workflow</strong>：代码仓库里的协作规则、任务记录和交付检查。</p>
            <p data-reveal><strong>Forge Harness</strong>：Agent 任务执行、工具调用和运行记录。</p>
            <p data-reveal><strong>本站</strong>：Go + Nuxt 博客，包含文章、搜索、评论和微信小程序。</p>
          </div>
        </section>

        <section data-section>
          <h2 data-reveal>文章和项目</h2>
          <div class="selected-grid">
            <div class="selected-group">
              <h3 data-reveal>代表项目</h3>
              <NuxtLink v-for="project in representativeProjects" :key="project.slug" data-reveal :to="`/projects/${encodeURIComponent(project.slug)}`">{{ project.name }} <span aria-hidden="true">↗</span></NuxtLink>
              <p v-if="projectsError" data-reveal class="text-sm text-[var(--text-muted)]">项目入口暂时无法加载。</p>
            </div>
            <div class="selected-group">
              <h3 data-reveal>代表文章</h3>
              <NuxtLink v-for="article in representativeArticles" :key="article.slug" data-reveal :to="`/posts/${encodeURIComponent(article.slug)}`">{{ article.title }} <span aria-hidden="true">↗</span></NuxtLink>
              <p v-if="articlesError" data-reveal class="text-sm text-[var(--text-muted)]">文章入口暂时无法加载。</p>
            </div>
          </div>
          <div v-if="about.representative_links?.length" class="mt-6 flex flex-wrap gap-4">
            <a v-for="link in about.representative_links.filter(item => safePortfolioHref(item.href))" :key="`${link.label}-${link.href}`" data-reveal :href="safePortfolioHref(link.href)" target="_blank" rel="noopener" class="inline-flex min-h-11 items-center text-sm text-[var(--accent)] hover:underline">{{ link.label }} ↗</a>
          </div>
        </section>

        <section data-section class="contact-chapter">
          <h2 data-reveal>联系我</h2>
          <div class="contact-actions">
            <a v-for="contact in (about.contacts ?? []).filter(item => safePortfolioHref(item.href, true))" :key="`${contact.label}-${contact.href}`" data-reveal :href="safePortfolioHref(contact.href, true)" :target="contact.href.startsWith('mailto:') ? undefined : '_blank'" rel="noopener" class="contact-button">
              <span>{{ contact.href.startsWith('mailto:') ? '给我发邮件' : contact.label === 'GitHub' ? '访问 GitHub' : contact.label }} <span aria-hidden="true">↗</span></span>
              <span class="contact-address">{{ contact.text }}</span>
            </a>
          </div>
        </section>
      </div>
    </AboutScrollReveal>
    <AboutWechatContact v-if="about" />
  </div>
</template>

<style scoped>
.about-focus-story { position: relative; isolation: isolate; max-width: 760px; margin: 0 auto; padding: 0 24px; }
.about-focus-story > section, .about-story { position: relative; z-index: 1; }
.about-focus-story :deep(.about-pointer-background) { position: fixed; z-index: 0; opacity: .55; }
section { padding: 48px 0; scroll-margin-top: 150px; }
.about-story > section { border-top: 1px solid var(--border-default); }
.about-intro { padding-top: 40px; }
h1 { margin: 22px 0 20px; font-size: clamp(30px, 4vw, 44px); line-height: 1.4; font-weight: 600; letter-spacing: -.03em; }
h2 { margin-bottom: 24px; font-size: clamp(23px, 3vw, 28px); font-weight: 550; line-height: 1.5; }
.about-intro p { margin: 16px 0; font-size: 16px; line-height: 2; color: var(--text-secondary); }
.scroll-hint { font-size: 13px; color: var(--text-muted); }
.scroll-hint { display: inline-flex; align-items: center; gap: 16px; min-height: 44px; margin-top: 24px; }
.scroll-hint:focus-visible { outline: 2px solid var(--accent); outline-offset: 5px; }
.about-portrait { width: 120px; height: 150px; object-fit: cover; border-radius: 16px; }
.reader-notes { display: grid; gap: 16px; font-size: 16px; line-height: 1.9; color: var(--text-secondary); }
.reader-notes strong { font-weight: 550; color: var(--text-primary); }
.selected-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 48px; }
.selected-group a { display: flex; justify-content: space-between; gap: 16px; min-height: 56px; padding: 16px 0; border-bottom: 1px solid var(--border-default); font-size: 16px; line-height: 1.7; overflow-wrap: anywhere; }
.selected-group a:hover { color: var(--accent); }
.contact-chapter { padding-bottom: 64px; }
.contact-actions { display: flex; flex-wrap: wrap; gap: 16px; }
.contact-button { display: flex; flex-direction: column; gap: 12px; min-height: 100px; min-width: 240px; max-width: 100%; padding: 22px 28px; border: 1px solid var(--border-strong); border-radius: 16px; background: var(--surface-page); transition: transform 250ms, border-color 250ms; }
.contact-button:hover { transform: translateY(-4px); border-color: var(--accent); }
.contact-button:focus-visible { outline: 2px solid var(--accent); outline-offset: 5px; }
.contact-button > span:first-child { display: flex; justify-content: space-between; gap: 24px; font-weight: 500; }
.contact-address { font-size: 12px; color: var(--text-muted); overflow-wrap: anywhere; }
@media (max-width: 640px) {
  .about-focus-story { padding-inline: 0; }
  section { padding-block: 32px; }
  .about-intro { padding-top: 40px; }
  .selected-grid { grid-template-columns: 1fr; gap: 32px; }
  .contact-actions { flex-direction: column; }
  .contact-button { min-width: 0; width: 100%; }
}
@media (max-width: 640px), (max-height: 760px) {
  :deep(.about-wechat-toggle) { bottom: 88px; }
}
@media (prefers-reduced-motion: reduce) { .contact-button { transition: none; } .contact-button:hover { transform: none; } }
</style>
