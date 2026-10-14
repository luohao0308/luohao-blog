<script setup lang="ts">
import { articleDate, formatDate, type Article } from '~/composables/useArticles'
import { articleSummary, readingMinutes } from '~/utils/article'

const props = defineProps<{
  article: Article
}>()

const summary = computed(() => articleSummary(props.article.summary, props.article.content_md))

</script>

<template>
  <article class="flex h-full min-w-0 flex-col overflow-hidden rounded-xl border border-slate-200 bg-white transition-colors hover:border-[#9aafc7] dark:border-slate-800 dark:bg-slate-950 dark:hover:border-slate-600">
    <NuxtLink v-if="article.cover_url" :to="`/posts/${encodeURIComponent(article.slug)}`" class="block overflow-hidden bg-slate-100 dark:bg-slate-900">
      <img :src="assetUrl(article.cover_url)" :alt="`${article.title}封面`" class="aspect-[16/9] w-full object-cover transition-transform duration-300 hover:scale-[1.02]">
    </NuxtLink>
    <div class="flex flex-1 flex-col p-6">
      <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500 dark:text-slate-400">
        <time :datetime="articleDate(article.published_at || article.created_at)">{{ formatDate(article.published_at || article.created_at) }}</time>
        <span>{{ readingMinutes(article.content_md) }} 分钟阅读</span>
      </div>
      <span v-if="article.featured" class="mt-3 w-fit rounded-full bg-amber-100 px-2 py-0.5 text-[11px] text-amber-700 dark:bg-amber-950 dark:text-amber-300">精选</span>
      <h2 class="mt-3 text-xl font-medium leading-relaxed text-slate-800 dark:text-slate-100">
        <NuxtLink :to="`/posts/${encodeURIComponent(article.slug)}`" class="break-words hover:text-[#3c5d85] dark:hover:text-blue-300">
          {{ article.title }}
        </NuxtLink>
      </h2>
      <p v-if="summary" class="mt-3 line-clamp-3 text-sm leading-7 text-slate-600 dark:text-slate-400">
        {{ summary }}
      </p>
      <div class="mt-auto flex flex-wrap items-center gap-2 pt-6 text-xs">
        <NuxtLink v-for="tag in article.tags" :key="tag" :to="`/tags/${encodeURIComponent(tag)}`" class="max-w-full break-words rounded-full bg-[#e7eee9] px-2 py-1 text-[#365f53] hover:underline dark:bg-emerald-950 dark:text-emerald-200">
          {{ tag }}
        </NuxtLink>
      </div>
    </div>
  </article>
</template>
