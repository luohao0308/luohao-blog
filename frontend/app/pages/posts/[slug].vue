<script setup lang="ts">
import { ARTICLE_STATUS_LABEL, formatDate, useArticleBySlug, usePublishedArticles } from '~/composables/useArticles'
import { isCollected, toggleCollection } from '~/composables/useCollections'

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
const { data: published, error: navigationError } = await usePublishedArticles()
const adjacent = computed(() => {
  const list = published.value ?? []
  const index = list.findIndex(item => item.slug === slug.value)
  return index < 0 ? {} : { newer: list[index - 1], older: list[index + 1] }
})

// Related articles rank shared category (weighted) and tags above the rest;
// when nothing overlaps, the latest articles fill in so the section doubles
// as "keep reading" rather than disappearing.
const related = computed(() => {
  const list = published.value ?? []
  const currentTags = article.value.tags ?? []
  const currentCategory = article.value.category?.slug
  const scored = list
    .filter(item => item.slug !== slug.value)
    .map((item) => {
      const shared = (item.tags ?? []).filter(tag => currentTags.includes(tag)).length
        + (item.category?.slug && item.category.slug === currentCategory ? 2 : 0)
      return { item, shared }
    })
  scored.sort((a, b) => b.shared - a.shared)
  const hits = scored.filter(entry => entry.shared > 0).slice(0, 3)
  return (hits.length ? hits : scored.slice(0, 3)).map(entry => entry.item)
})

// Like state mirrors the server's per-client dedup: localStorage remembers
// the like so the button reflects this browser, while the server enforces
// the real 24h window. Optimistic on click; the displayed count reads SSR
// data plus the local bump.
const liked = ref(false)
const likeCount = ref(article.value.like_count ?? 0)
const collected = ref(false)

onMounted(() => {
  liked.value = localStorage.getItem(`blog:liked:${slug.value}`) === '1'
  collected.value = isCollected(slug.value)
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

function toggleCollect() {
  collected.value = toggleCollection(slug.value, article.value.title)
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

useHead({ title: article.value.title })
</script>

<template>
  <article class="space-y-6">
    <header class="space-y-5 border-b border-slate-200 pb-8 dark:border-slate-800">
      <NuxtLink to="/posts" class="text-xs text-[#3c5d85] hover:underline dark:text-blue-300">返回全部文章</NuxtLink>
      <h1 class="break-words text-3xl font-medium leading-relaxed">{{ article.title }}</h1>
      <div class="flex flex-wrap items-center gap-3 text-sm text-slate-500 dark:text-slate-400">
        <time>{{ formatDate(article.published_at || article.created_at) }}</time>
        <span aria-label="阅读量">{{ article.view_count }} 次阅读</span>
        <button
          type="button"
          class="rounded px-2 py-1 text-xs transition-colors"
          :class="liked ? 'bg-rose-100 text-rose-600 dark:bg-rose-950 dark:text-rose-300' : 'bg-slate-100 text-slate-600 hover:bg-rose-50 dark:bg-slate-900 dark:text-slate-300 dark:hover:bg-rose-950/40'"
          :aria-pressed="liked"
          @click="like"
        >{{ liked ? '♥' : '♡' }} 点赞 {{ likeCount }}</button>
        <button
          type="button"
          class="rounded px-2 py-1 text-xs transition-colors"
          :class="collected ? 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300' : 'bg-slate-100 text-slate-600 hover:bg-amber-50 dark:bg-slate-900 dark:text-slate-300 dark:hover:bg-amber-950/40'"
          :aria-pressed="collected"
          @click="toggleCollect"
        >{{ collected ? '★ 已收藏' : '☆ 收藏' }}</button>
        <span
          v-if="article.status !== 2"
          class="rounded bg-amber-100 px-1.5 py-0.5 text-xs text-amber-700"
        >{{ ARTICLE_STATUS_LABEL[article.status] }}</span>
        <NuxtLink v-for="tag in article.tags" :key="tag" :to="`/tags/${encodeURIComponent(tag)}`" class="max-w-full break-words rounded bg-[#e7eee9] px-2 py-1 text-xs text-[#365f53] hover:underline dark:bg-emerald-950 dark:text-emerald-200">
          {{ tag }}
        </NuxtLink>
      </div>
    </header>

    <!-- eslint-disable-next-line vue/no-v-html -- trusted backend-rendered HTML -->
    <div class="markdown-body space-y-4 leading-7" v-html="article.content_html" />
    <ChatWindow />
    <CommentSection :slug="slug" />
    <section v-if="related.length" class="space-y-4 border-t border-slate-200 pt-8 dark:border-slate-800">
      <h2 class="text-lg font-medium text-slate-800 dark:text-slate-100">相关文章</h2>
      <div class="grid gap-4 sm:grid-cols-3">
        <NuxtLink
          v-for="item in related"
          :key="item.id"
          :to="`/posts/${encodeURIComponent(item.slug)}`"
          class="rounded-lg border border-slate-200 p-4 transition-shadow hover:shadow-md dark:border-slate-800"
        >
          <h3 class="line-clamp-2 text-sm font-medium">{{ item.title }}</h3>
          <p class="mt-2 text-xs text-slate-500 dark:text-slate-400">
            {{ (item.tags ?? [])[0] ?? '随笔' }} · {{ formatDate(item.published_at || item.created_at) }}
          </p>
        </NuxtLink>
      </div>
    </section>

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
