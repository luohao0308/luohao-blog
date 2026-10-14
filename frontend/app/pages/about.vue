<script setup lang="ts">
import { usePublishedArticles } from '~/composables/useArticles'

const { data: about, status, error, refresh } = await useAboutProfile()
const { data: projects, error: projectsError } = await usePublicProjects()
const { data: articles, error: articlesError } = await usePublishedArticles()

const representativeProjects = computed(() => {
  const list = projects.value ?? []
  const featured = list.filter(project => project.featured)
  return (featured.length ? featured : list).slice(0, 2)
})
const representativeArticles = computed(() => (articles.value ?? []).slice(0, 2))
usePageSeo({
  title: () => about.value?.name ? `关于 ${about.value.name}` : '关于我',
  description: () => about.value?.intro || about.value?.headline,
  image: () => about.value?.profile_image,
})
</script>

<template>
  <div class="space-y-12">
    <div v-if="error" role="alert" class="space-y-3 py-20 text-center">
      <p class="text-sm text-slate-600 dark:text-slate-400">关于页面暂时无法加载。</p>
      <button type="button" class="text-sm text-[#3c5d85] underline dark:text-blue-300" @click="refresh()">重新加载</button>
    </div>
    <p v-else-if="status === 'pending'" role="status" class="py-20 text-center text-sm text-slate-500">正在加载关于页面…</p>
    <template v-else-if="about">
      <section class="grid gap-8 border-b border-slate-200 pb-10 sm:grid-cols-[minmax(0,1fr)_12rem] sm:items-center">
        <div class="space-y-5">
          <p class="text-xs font-medium uppercase tracking-[0.2em] text-[#3c5d85] dark:text-blue-300">关于我</p>
          <h1 class="text-3xl font-semibold leading-relaxed sm:text-4xl">{{ about.name || about.headline }}</h1>
          <p v-if="about.name" class="text-lg leading-8 text-slate-700 dark:text-slate-300">{{ about.headline }}</p>
          <p class="whitespace-pre-line text-sm leading-8 text-slate-600 dark:text-slate-400">{{ about.intro }}</p>
        </div>
        <img v-if="about.profile_image" :src="assetUrl(about.profile_image)" :alt="`${about.name || '个人'} 头像`" class="aspect-square w-full rounded-2xl border border-slate-200 object-cover dark:border-slate-800">
      </section>

      <section v-if="about.current_focus" class="rounded-xl bg-[#eef3f8] p-6 dark:bg-slate-900">
        <h2 class="text-sm font-medium text-[#3c5d85] dark:text-blue-300">当前方向</h2>
        <p class="mt-3 whitespace-pre-line text-base leading-8 text-slate-700 dark:text-slate-300">{{ about.current_focus }}</p>
      </section>

      <section v-if="about.skills?.length" class="space-y-5">
        <h2 class="text-xl font-bold tracking-tight">技能栈</h2>
        <div class="space-y-4">
          <div v-for="group in about.skills" :key="group.group" class="flex flex-wrap items-baseline gap-2">
            <span class="w-20 shrink-0 text-sm font-medium text-slate-500 dark:text-slate-400">{{ group.group }}</span>
            <span v-for="item in group.items" :key="item" class="rounded-full border border-slate-200 px-2.5 py-0.5 text-sm dark:border-slate-700">{{ item }}</span>
          </div>
        </div>
      </section>

      <section v-if="about.timeline?.length" class="space-y-5">
        <h2 class="text-xl font-bold tracking-tight">经历</h2>
        <ol class="space-y-5 border-l-2 border-slate-200 pl-5 dark:border-slate-700">
          <li v-for="(item, index) in about.timeline" :key="`${item.year}-${index}`" class="relative">
            <span class="absolute -left-[27px] top-1.5 h-3 w-3 rounded-full border-2 border-white bg-blue-600 dark:border-slate-950" />
            <p class="text-sm font-medium text-blue-600 dark:text-blue-400">{{ item.year }}</p>
            <p class="whitespace-pre-line leading-7 text-slate-700 dark:text-slate-300">{{ item.text }}</p>
          </li>
        </ol>
      </section>

      <section class="space-y-5">
        <h2 class="text-xl font-bold tracking-tight">代表内容</h2>
        <div class="grid gap-5 sm:grid-cols-2">
          <div class="space-y-3 rounded-xl border border-slate-200 p-5 dark:border-slate-800">
            <h3 class="text-sm font-medium">代表项目</h3>
            <NuxtLink v-for="project in representativeProjects" :key="project.slug" :to="`/projects/${encodeURIComponent(project.slug)}`" class="block text-sm text-[#3c5d85] hover:underline dark:text-blue-300">{{ project.name }} →</NuxtLink>
            <p v-if="projectsError" class="text-xs text-slate-500">项目入口暂时无法加载。</p>
          </div>
          <div class="space-y-3 rounded-xl border border-slate-200 p-5 dark:border-slate-800">
            <h3 class="text-sm font-medium">代表文章</h3>
            <NuxtLink v-for="article in representativeArticles" :key="article.slug" :to="`/posts/${encodeURIComponent(article.slug)}`" class="block text-sm text-[#3c5d85] hover:underline dark:text-blue-300">{{ article.title }} →</NuxtLink>
            <p v-if="articlesError" class="text-xs text-slate-500">文章入口暂时无法加载。</p>
          </div>
        </div>
        <div v-if="about.representative_links?.length" class="flex flex-wrap gap-2">
          <a v-for="link in about.representative_links.filter(item => safePortfolioHref(item.href))" :key="`${link.label}-${link.href}`" :href="safePortfolioHref(link.href)" target="_blank" rel="noopener" class="rounded-full border border-slate-200 px-3 py-1.5 text-sm hover:border-blue-400 hover:text-blue-600 dark:border-slate-700 dark:hover:text-blue-300">{{ link.label }} ↗</a>
        </div>
      </section>

      <section v-if="about.contacts?.length" class="space-y-5">
        <h2 class="text-xl font-bold tracking-tight">联系我</h2>
        <ul class="space-y-3">
          <li v-for="contact in about.contacts.filter(item => safePortfolioHref(item.href, true))" :key="`${contact.label}-${contact.href}`" class="text-slate-700 dark:text-slate-300">
            <span class="mr-2 inline-block w-20 text-sm text-slate-500 dark:text-slate-400">{{ contact.label }}</span>
            <a :href="safePortfolioHref(contact.href, true)" target="_blank" rel="noopener" class="break-all hover:text-blue-600 hover:underline dark:hover:text-blue-400">{{ contact.text }}</a>
          </li>
        </ul>
      </section>
    </template>
  </div>
</template>
