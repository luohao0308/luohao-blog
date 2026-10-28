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
    <AboutContinuousStory v-else-if="about">
      <AboutNarrative continuous data-journey-scene name="罗浩" headline="" :intro="displayIntro" :image="about.profile_image" />
      <div data-about-content class="about-story">
        <AboutChapter data-journey-scene>
          <h2 data-chapter-item>这里会写什么</h2>
          <div class="reader-notes" data-chapter-item>
            <p><strong>产品实践</strong>：记录从想法到实现的过程、需求取舍与使用反馈。</p>
            <p><strong>AI 工具与工作流</strong>：分享日常尝试，聊聊工具如何融入创作与开发。</p>
            <p><strong>技术复盘</strong>：把踩过的坑、做过的判断和新发现整理下来。</p>
          </div>
        </AboutChapter>

        <AboutChapter data-journey-scene>
          <h2 data-chapter-item>正在做的事</h2>
          <div class="reader-notes" data-chapter-item>
            <p><strong>dev-workflow</strong>：探索人与 AI 协作开发的流程，边使用边记录。</p>
            <p><strong>Forge Harness</strong>：尝试把 AI 开发中的想法做成工具，分享过程中的取舍与发现。</p>
            <p><strong>本站</strong>：持续整理文章与项目，把探索过程留下来，也欢迎读者交流自己的尝试。</p>
          </div>
        </AboutChapter>

        <AboutChapter data-journey-scene>
          <h2 data-chapter-item>从这里开始</h2>
          <div class="selected-grid">
            <div class="selected-group" data-chapter-item>
              <h3>代表项目</h3>
              <NuxtLink v-for="project in representativeProjects" :key="project.slug" :to="`/projects/${encodeURIComponent(project.slug)}`">{{ project.name }} <span aria-hidden="true">↗</span></NuxtLink>
              <p v-if="projectsError" class="text-sm text-[var(--text-muted)]">项目入口暂时无法加载。</p>
            </div>
            <div class="selected-group" data-chapter-item>
              <h3>代表文章</h3>
              <NuxtLink v-for="article in representativeArticles" :key="article.slug" :to="`/posts/${encodeURIComponent(article.slug)}`">{{ article.title }} <span aria-hidden="true">↗</span></NuxtLink>
              <p v-if="articlesError" class="text-sm text-[var(--text-muted)]">文章入口暂时无法加载。</p>
            </div>
          </div>
          <div v-if="about.representative_links?.length" class="mt-6 flex flex-wrap gap-4" data-chapter-item>
            <a v-for="link in about.representative_links.filter(item => safePortfolioHref(item.href))" :key="`${link.label}-${link.href}`" :href="safePortfolioHref(link.href)" target="_blank" rel="noopener" class="inline-flex min-h-11 items-center text-sm text-[var(--accent)] hover:underline">{{ link.label }} ↗</a>
          </div>
        </AboutChapter>

        <AboutChapter data-journey-scene class="contact-chapter">
          <h2 data-chapter-item>一起交流</h2>
          <p class="chapter-lead" data-chapter-item>如果你也在做产品、写代码或尝试 AI，欢迎分享你的问题、想法和实践。</p>
          <div class="contact-actions" data-chapter-item>
            <a v-for="contact in (about.contacts ?? []).filter(item => safePortfolioHref(item.href, true))" :key="`${contact.label}-${contact.href}`" :href="safePortfolioHref(contact.href, true)" :target="contact.href.startsWith('mailto:') ? undefined : '_blank'" rel="noopener" class="contact-button">
              <span>{{ contact.href.startsWith('mailto:') ? '给我发邮件' : contact.label === 'GitHub' ? '访问 GitHub' : contact.label }} <span aria-hidden="true">↗</span></span>
              <span class="contact-address">{{ contact.text }}</span>
            </a>
          </div>
        </AboutChapter>
      </div>
    </AboutContinuousStory>
    <AboutWechatContact v-if="about" />
  </div>
</template>

<style scoped>
.about-story { position: relative; padding: 0 0 64px 32px; border-left: 1px solid color-mix(in srgb, #c3aa7c 45%, transparent); }
.about-chapter { position: relative; }
.about-chapter::before { content: ""; position: absolute; left: -36px; top: clamp(48px, 8vw, 100px); width: 7px; height: 7px; border-radius: 50%; background: #c3aa7c; }
.about-chapter { padding: clamp(48px, 8vw, 100px) 0; border-top: 1px solid var(--border-default); }
.chapter-label { margin-bottom: 16px; font-size: 10px; letter-spacing: .2em; color: var(--text-muted); }
h2 { margin-bottom: 40px; font-size: 26px; font-weight: 550; letter-spacing: -.04em; }
.chapter-lead { max-width: 760px; font-size: 16px; line-height: 1.9; }
.reader-notes { display: grid; gap: 16px; max-width: 720px; font-size: 16px; line-height: 1.9; }
.reader-notes strong { font-weight: 550; }
.selected-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 48px; }
.selected-group a { display: flex; justify-content: space-between; gap: 16px; min-height: 56px; padding: 16px 0; border-bottom: 1px solid var(--border-default); font-size: 16px; line-height: 1.7; overflow-wrap: anywhere; }
.selected-group a:hover { color: var(--accent); }
.contact-chapter { position: relative; }
.contact-chapter::before { content: ''; position: absolute; z-index: -1; inset: 0; background: radial-gradient(ellipse at 30% 60%, rgb(222 181 104 / 9%), transparent 70%); }
.contact-actions { display: flex; flex-wrap: wrap; gap: 16px; }
.contact-button { display: flex; flex-direction: column; gap: 12px; min-height: 100px; min-width: 240px; max-width: 100%; padding: 22px 28px; border: 1px solid var(--border-strong); border-radius: 16px; background: var(--surface-page); transition: transform 250ms, border-color 250ms; }
.contact-button:hover { transform: translateY(-4px); border-color: var(--accent); }
.contact-button:focus-visible { outline: 2px solid var(--accent); outline-offset: 5px; }
.contact-button > span:first-child { display: flex; justify-content: space-between; gap: 24px; font-weight: 500; }
.contact-address { font-size: 12px; color: var(--text-muted); overflow-wrap: anywhere; }
@media (max-width: 640px) {
  .selected-grid { grid-template-columns: 1fr; gap: 32px; }
  .contact-actions { flex-direction: column; }
  .contact-button { min-width: 0; width: 100%; }
  h2 { margin-bottom: 32px; }
}
@media (max-width: 640px), (max-height: 760px) {
  :deep(.about-wechat-toggle) { bottom: 88px; }
}
@media (prefers-reduced-motion: reduce) { .contact-button { transition: none; } .contact-button:hover { transform: none; } }
/* Scope reading sizes to this page, including continuous-camera overrides. */
.about-page :deep(.continuous-story .about-narrative .narrative-opening) { min-height: 0; padding: 24px 0 0; gap: 20px; justify-content: flex-start; }
.about-page :deep(.continuous-story .about-narrative .narrative-opening h1) { font-size: clamp(28px, 4vw, 48px); margin: 12px 0; }
.about-page :deep(.continuous-story .about-narrative .narrative-greeting) { font-size: 16px; }
.about-page :deep(.continuous-story .about-narrative .narrative-scroll) { position: static; margin-top: 0; }
.about-page :deep(.continuous-story .about-narrative .narrative-introduction) { padding-top: 24px; margin-inline: 0; }
.about-page :deep(.continuous-story .about-narrative .narrative-prose p) { font-size: 16px; }
.about-page :deep(.continuous-story h2) { font-size: 26px; margin-bottom: 24px; }
.about-page :deep(.continuous-story .selected-group a) { font-size: 16px; }
.contact-chapter .chapter-lead { margin-bottom: 24px; }
.about-page :deep(.continuous-story .about-chapter) { padding: 32px 0; min-height: 0; }
</style>
