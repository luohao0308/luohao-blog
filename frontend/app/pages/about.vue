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
const displayIntro = computed(() => (about.value?.intro ?? '').replace(/相信工程治理和自动化流程是个人开发者的杠杆[。.!！]?/g, '').trim())
const representativeArticles = computed(() => (articles.value ?? []).slice(0, 2))
usePageSeo({
  title: () => about.value?.name ? `关于 ${about.value.name}` : '关于我',
  description: () => displayIntro.value || about.value?.headline,
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
      <AboutNarrative continuous data-journey-scene :name="about.name || '罗浩'" :headline="about.headline"  :intro="displayIntro" :skills="about.skills" :image="about.profile_image" />
      <div data-about-content class="about-story">
        <AboutChapter v-if="about.current_focus" data-journey-scene>
          <p class="chapter-label" data-chapter-item>此刻 / NOW</p>
          <h2 data-chapter-item>当前方向</h2>
          <p class="chapter-lead whitespace-pre-line" data-chapter-item>{{ about.current_focus }}</p>
        </AboutChapter>

        <AboutChapter v-if="about.skills?.length" data-journey-scene>
          <p class="chapter-label" data-chapter-item>01 / TOOLKIT</p>
          <h2 data-chapter-item>技能栈</h2>
          <div class="skills-grid">
            <div v-for="(group, groupIndex) in about.skills" :key="group.group" class="skill-group" data-chapter-item>
              <span class="skill-number" aria-hidden="true">0{{ groupIndex + 1 }}</span>
              <h3>{{ group.group }}</h3>
              <ul><li v-for="item in group.items" :key="item">{{ item }}</li></ul>
            </div>
          </div>
        </AboutChapter>

        <AboutChapter v-if="about.timeline?.length" data-journey-scene>
          <p class="chapter-label" data-chapter-item>02 / JOURNEY</p>
          <h2 data-chapter-item>经历</h2>
          <ol class="journey-list">
            <li v-for="(item, index) in about.timeline" :key="`${item.year}-${index}`" data-chapter-item>
              <p class="journey-year">{{ item.year }}</p>
              <p class="journey-text whitespace-pre-line">{{ item.text }}</p>
            </li>
          </ol>
        </AboutChapter>

        <AboutChapter data-journey-scene>
          <p class="chapter-label" data-chapter-item>03 / SELECTED</p>
          <h2 data-chapter-item>代表内容</h2>
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

        <AboutChapter v-if="about.contacts?.length" data-journey-scene class="contact-chapter">
          <p class="chapter-label" data-chapter-item>04 / CONNECT</p>
          <h2 data-chapter-item>联系我</h2>
          <div class="contact-actions" data-chapter-item>
            <a v-for="contact in about.contacts.filter(item => safePortfolioHref(item.href, true))" :key="`${contact.label}-${contact.href}`" :href="safePortfolioHref(contact.href, true)" :target="contact.href.startsWith('mailto:') ? undefined : '_blank'" rel="noopener" class="contact-button">
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
h2 { margin-bottom: 40px; font-size: clamp(28px, 4vw, 44px); font-weight: 550; letter-spacing: -.04em; }
.chapter-lead { max-width: 760px; font-size: clamp(20px, 2.5vw, 28px); line-height: 1.9; }
.skills-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 36px; }
.skill-group { position: relative; padding: 28px 24px 32px; border: 1px solid var(--border-default); border-radius: 20px; background: linear-gradient(145deg, color-mix(in srgb, #c3aa7c 7%, var(--surface-page)), var(--surface-page) 75%); transform-origin: center bottom; }
.skill-number { display: block; margin-bottom: 32px; font-size: 11px; color: var(--text-muted); letter-spacing: .2em; }
.skill-group h3, .selected-group h3 { margin-bottom: 24px; font-size: 14px; font-weight: 500; color: var(--text-muted); }
.skill-group ul { display: flex; flex-wrap: wrap; align-content: start; gap: 10px 8px; }
.skill-group li:first-child { flex-basis: 100%; padding: 0 0 16px; background: none; font-size: 36px; font-weight: 500; letter-spacing: -.04em; }
.skill-group li { max-width: 100%; overflow-wrap: anywhere; padding: 7px 12px; border-radius: 8px; background: color-mix(in srgb, var(--accent) 5%, var(--surface-page)); font-size: 14px; }
.journey-list li { position: relative; display: grid; grid-template-columns: 140px minmax(0, 1fr); gap: 32px; padding: 32px 0; border-bottom: 1px solid var(--border-default); }
.journey-list li:first-child { padding-top: 0; }
.journey-year { font-size: clamp(44px, 6vw, 72px); font-weight: 300; line-height: 1.2; letter-spacing: -.05em; color: var(--text-muted); }
.journey-text { font-size: clamp(17px, 2vw, 21px); line-height: 1.9; text-wrap: pretty; }
.selected-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 48px; }
.selected-group a { display: flex; justify-content: space-between; gap: 16px; min-height: 56px; padding: 16px 0; border-bottom: 1px solid var(--border-default); font-size: 17px; line-height: 1.7; overflow-wrap: anywhere; }
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
  .skills-grid, .selected-grid { grid-template-columns: 1fr; gap: 32px; }
  .journey-list li { grid-template-columns: 1fr; gap: 16px; }
  .contact-actions { flex-direction: column; }
  .contact-button { min-width: 0; width: 100%; }
  h2 { margin-bottom: 32px; }
}
@media (max-width: 640px), (max-height: 760px) {
  :deep(.about-wechat-toggle) { bottom: 88px; }
}
@media (prefers-reduced-motion: reduce) { .contact-button { transition: none; } .contact-button:hover { transform: none; } }
</style>
