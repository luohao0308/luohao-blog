<script setup lang="ts">
import { articleImages } from '~/data/editorial'
import { projectArticleSlugs } from '~/data/projectArticles'
import { ARTICLE_STATUS_LABEL, formatDate, useArticleBySlug, usePublishedArticles } from '~/composables/useArticles'
import { articleSummary, prepareArticleHtml, readingMinutes } from '~/utils/article'

const route = useRoute()
definePageMeta({ key: route => route.path })
const slug = computed(() => String(route.params.slug))

const { data, error } = await useArticleBySlug(slug.value)

if (error.value) {
  throw createError({ statusCode: error.value.statusCode === 404 ? 404 : 503, message: error.value.statusCode === 404 ? '文章不存在' : '文章暂时无法加载' })
}
if (!data.value) throw createError({ statusCode: 404, message: '文章不存在' })

// Non-null by the 404 guard above; the backend renderer owns the HTML.
const article = computed(() => data.value!)
const editorialCover = computed(() => article.value.cover_url ? undefined : articleImages[article.value.slug])
const coverUrl = computed(() => editorialCover.value?.src || assetUrl(article.value.cover_url))
const coverAlt = computed(() => editorialCover.value?.alt || `${article.value.title}封面`)
const failedCover = ref('')
const coverImage = ref<HTMLImageElement | null>(null)
const showCover = computed(() => article.value.cover_url && coverUrl.value && coverUrl.value !== failedCover.value)
onMounted(() => {
  if (coverImage.value?.complete && !coverImage.value.naturalWidth) failedCover.value = coverUrl.value
})
const preparedContent = computed(() => prepareArticleHtml(article.value.content_html ?? ''))
const minutes = computed(() => readingMinutes(article.value.content_md))
const summary = computed(() => articleSummary(article.value.summary, article.value.content_md))
const { data: published, error: navigationError } = await usePublishedArticles()
const { data: publicProjects, error: projectsError, refresh: refreshProjects } = await usePublicProjects()
const relatedProjects = computed(() => article.value.status === 2
  ? (publicProjects.value ?? []).filter(project => projectArticleSlugs[project.slug]?.includes(slug.value))
  : [])
const hasProjectRelation = computed(() => Object.values(projectArticleSlugs).some(slugs => slugs.includes(slug.value)))
const adjacent = computed(() => {
  const list = published.value ?? []
  const index = list.findIndex(item => item.slug === slug.value)
  return index < 0 ? {} : { newer: list[index - 1], older: list[index + 1] }
})

// Like state mirrors the server's per-client dedup: localStorage remembers
// the like so the button reflects this browser, while the server enforces
// the real 24h window. Optimistic on click; the displayed count reads SSR
// data plus the local bump.
const liked = ref(false)
const likeCount = ref(article.value.like_count ?? 0)
const collections = useCollections()
const collected = computed(() => collections.collected(slug.value))
const collectionError = ref('')
const collectionStatusError = computed(() => collectionError.value || collections.error.value)

onMounted(() => {
  liked.value = localStorage.getItem(`blog:liked:${slug.value}`) === '1'
  collections.initialize(true).then((ok) => { if (!ok) collectionError.value = '收藏状态暂时无法加载。' }).catch(() => { collectionError.value = '收藏状态暂时无法加载。' })
})

async function like() {
  if (liked.value)
    return
  liked.value = true
  likeCount.value++
  localStorage.setItem(`blog:liked:${slug.value}`, '1')
  try {
    // Same content-type contract as the view report: the backend binds the
    // (empty) JSON body and rejects bodyless posts without the header.
    await $fetch(`/api/v1/articles/${encodeURIComponent(slug.value)}/like`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    })
  }
  catch {
    // Keep the local state: the server dedups by client anyway and the
    // count reconciles on the next SSR read.
  }
}

async function toggleCollect() {
  collectionError.value = ''
  try { await collections.toggle(slug.value, article.value.title) }
  catch { collectionError.value = '收藏操作失败，请重试。' }
}

async function retryCollections() {
  collectionError.value = ''
  const ok = await collections.initialize(true)
  if (!ok) collectionError.value = '收藏状态暂时无法加载。'
}

// Report one view per page open. Fire-and-forget: the server deduplicates by
// client identity within 24h, and the counter the page shows is the one read
// during SSR (this visit shows up on the next look).
onMounted(() => {
  $fetch(`/api/v1/articles/${encodeURIComponent(slug.value)}/view`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  }).catch(() => {})
})

usePageSeo({
  title: () => article.value.title,
  description: () => summary.value,
  image: () => editorialCover.value?.src || article.value.cover_url,
  type: 'article',
})
</script>

<template>
  <article data-article-detail class="mx-auto w-full max-w-[860px] space-y-6">
    <header class="space-y-5 border-b border-slate-200 pb-8 dark:border-slate-800">
      <NuxtLink to="/posts" class="detail-button">← 返回全部文章</NuxtLink>
      <h1 tabindex="-1" class="break-words text-3xl font-medium leading-relaxed">{{ article.title }}</h1>
      <p v-if="summary" class="max-w-3xl text-base leading-8 text-slate-600 dark:text-slate-300">
        {{ summary }}
      </p>
      <div class="flex flex-wrap items-center gap-3 text-sm text-slate-500 dark:text-slate-400">
        <time>{{ formatDate(article.published_at || article.created_at) }}</time>
        <span v-if="minutes">约 {{ minutes }} 分钟阅读</span>
        <span aria-label="阅读量">{{ article.view_count }} 次阅读</span>
        <button
          type="button"
          class="rounded-full px-2.5 py-1 text-xs transition-colors"
          :class="liked ? 'bg-rose-100 text-rose-600 dark:bg-rose-950 dark:text-rose-300' : 'bg-slate-100 text-slate-600 hover:bg-rose-50 dark:bg-slate-900 dark:text-slate-300 dark:hover:bg-rose-950/40'"
          :aria-pressed="liked"
          @click="like"
        >{{ liked ? '♥' : '♡' }} 点赞 {{ likeCount }}</button>
        <button
          type="button"
          class="rounded-full px-2.5 py-1 text-xs transition-colors"
          :class="collected ? 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300' : 'bg-slate-100 text-slate-600 hover:bg-amber-50 dark:bg-slate-900 dark:text-slate-300 dark:hover:bg-amber-950/40'"
          :aria-pressed="collected"
          :disabled="!collections.ready.value || collections.busy.value.has(slug)"
          @click="toggleCollect"
        >{{ collected ? '★ 已收藏' : '☆ 收藏' }}</button>
        <span v-if="collectionStatusError" role="alert" class="text-xs text-red-600">{{ collectionStatusError }} <button type="button" class="underline" @click="retryCollections">重试</button></span>
        <span
          v-if="article.status !== 2"
          class="rounded-full bg-amber-100 px-1.5 py-0.5 text-xs text-amber-700"
        >{{ ARTICLE_STATUS_LABEL[article.status] }}</span>
        <NuxtLink v-for="tag in article.tags" :key="tag" :to="`/tags/${encodeURIComponent(tag)}`" class="max-w-full break-words rounded-full bg-[#e7eee9] px-2 py-1 text-xs text-[#365f53] hover:underline dark:bg-emerald-950 dark:text-emerald-200">
          {{ tag }}
        </NuxtLink>
      </div>
      <img
        v-if="showCover"
        ref="coverImage"
        :src="coverUrl"
        :alt="coverAlt"
        width="1280"
        height="720"
        class="detail-media rounded-xl border border-[var(--border-default)]"
        @error="failedCover = coverUrl"
      >
    </header>

    <ArticleContent :html="preparedContent.html" :headings="preparedContent.headings" />
    <section v-if="relatedProjects.length || (hasProjectRelation && projectsError)" aria-label="相关项目" class="space-y-4 border-t border-[var(--border-default)] py-6">
      <h2 class="text-sm text-[var(--text-muted)]">项目背景</h2>
      <div v-if="projectsError" role="alert" class="text-sm text-[var(--text-muted)]">相关项目暂时加载失败。<button type="button" class="ml-2 text-[var(--accent)] underline" @click="refreshProjects()">重新加载</button></div>
      <NuxtLink v-for="item in relatedProjects" v-else :key="item.slug" :to="`/projects/${encodeURIComponent(item.slug)}`" class="detail-button">
        <span class="flex items-center justify-between gap-3 font-medium">{{ item.name }} <span aria-hidden="true">↗</span></span>
      </NuxtLink>
    </section>
    <ChatWindow />
    <CommentSection :slug="slug" />
    <nav v-if="adjacent.newer || adjacent.older" aria-label="相邻文章" class="grid gap-6 border-t border-slate-200 pt-8 sm:grid-cols-2 dark:border-slate-800">
      <div>
        <NuxtLink v-if="adjacent.newer" :to="`/posts/${encodeURIComponent(adjacent.newer.slug)}`" class="block space-y-2 hover:text-[#3c5d85] dark:hover:text-blue-300">
          <span class="text-xs text-slate-500 dark:text-slate-400">上一篇 · 较新</span>
          <p class="break-words text-sm leading-7">{{ adjacent.newer.title }}</p>
        </NuxtLink>
      </div>
      <div class="sm:text-right">
        <NuxtLink v-if="adjacent.older" :to="`/posts/${encodeURIComponent(adjacent.older.slug)}`" class="block space-y-2 hover:text-[#3c5d85] dark:hover:text-blue-300">
          <span class="text-xs text-slate-500 dark:text-slate-400">下一篇 · 较早</span>
          <p class="break-words text-sm leading-7">{{ adjacent.older.title }}</p>
        </NuxtLink>
      </div>
    </nav>
    <p v-if="navigationError" class="text-sm text-slate-500 dark:text-slate-400">相邻文章暂时无法加载。</p>
  </article>
</template>
