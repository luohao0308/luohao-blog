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
const displayIntro = '我是罗浩，在这里记录做产品、写代码和折腾 AI 的过程。把一个想法做出来，也把中间的判断、踩坑和发现留下来。'
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
        <p data-reveal class="intro-label">关于我</p>
        <h1 data-reveal>你好，我是罗浩。</h1>
        <p data-reveal class="intro-copy">做产品，写代码，也折腾 AI。</p>
        <p data-reveal>{{ displayIntro }}</p>
        <img v-if="about.profile_image" data-reveal :src="assetUrl(about.profile_image)" alt="罗浩头像" width="120" height="150" class="about-portrait">
        <a data-reveal href="#about-writing" class="scroll-hint">向下滚动，继续了解 <span aria-hidden="true">↓</span></a>
      </section>
      <div data-about-content class="about-story">
        <section id="about-writing" data-section>
          <h2 data-reveal>这里会写什么</h2>
          <div class="reader-notes">
            <p data-reveal><strong>产品实践</strong>：记录从想法到实现的过程、需求取舍与使用反馈。</p>
            <p data-reveal><strong>AI 工具与工作流</strong>：分享日常尝试，聊聊工具如何融入创作与开发。</p>
            <p data-reveal><strong>技术复盘</strong>：把踩过的坑、做过的判断和新发现整理下来。</p>
          </div>
        </section>

        <section data-section>
          <h2 data-reveal>正在做的事</h2>
          <div class="reader-notes">
            <p data-reveal><strong>dev-workflow</strong>：探索人与 AI 协作开发的流程，边使用边记录。</p>
            <p data-reveal><strong>Forge Harness</strong>：尝试把 AI 开发中的想法做成工具，分享过程中的取舍与发现。</p>
            <p data-reveal><strong>本站</strong>：持续整理文章与项目，把探索过程留下来，也欢迎读者交流自己的尝试。</p>
          </div>
        </section>

        <section data-section>
          <h2 data-reveal>从这里开始</h2>
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
          <h2 data-reveal>一起交流</h2>
          <p data-reveal class="chapter-lead">如果你也在做产品、写代码或尝试 AI，欢迎分享你的问题、想法和实践。</p>
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
.about-intro .intro-label, .scroll-hint { font-size: 13px; color: var(--text-muted); }
.about-intro .intro-copy { font-size: 20px; color: var(--text-primary); }
.scroll-hint { display: inline-flex; align-items: center; gap: 16px; min-height: 44px; margin-top: 24px; }
.scroll-hint:focus-visible { outline: 2px solid var(--accent); outline-offset: 5px; }
.about-portrait { width: 120px; height: 150px; object-fit: cover; border-radius: 16px; }
.chapter-lead { font-size: 16px; line-height: 1.9; margin-bottom: 24px; }
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
  .about-intro .intro-copy { font-size: 18px; }
  .selected-grid { grid-template-columns: 1fr; gap: 32px; }
  .contact-actions { flex-direction: column; }
  .contact-button { min-width: 0; width: 100%; }
}
@media (max-width: 640px), (max-height: 760px) {
  :deep(.about-wechat-toggle) { bottom: 88px; }
}
@media (prefers-reduced-motion: reduce) { .contact-button { transition: none; } .contact-button:hover { transform: none; } }
</style>
