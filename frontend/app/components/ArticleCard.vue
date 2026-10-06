<script setup lang="ts">
import { articleDate, formatDate, type Article } from '~/composables/useArticles'

defineProps<{
  article: Article
}>()

</script>

<template>
  <article class="flex h-full min-w-0 flex-col rounded-xl border border-slate-200 bg-white p-6 transition-colors hover:border-[#9aafc7] dark:border-slate-800 dark:bg-slate-950 dark:hover:border-slate-600">
    <time :datetime="articleDate(article.published_at || article.created_at)" class="text-xs text-slate-500 dark:text-slate-400">{{ formatDate(article.published_at || article.created_at) }}</time>
    <h2 class="mt-4 text-xl font-medium leading-relaxed text-slate-800 dark:text-slate-100">
      <NuxtLink :to="`/posts/${encodeURIComponent(article.slug)}`" class="break-words hover:text-[#3c5d85] dark:hover:text-blue-300">
        {{ article.title }}
      </NuxtLink>
    </h2>
    <p v-if="article.summary" class="mt-3 line-clamp-3 text-sm leading-7 text-slate-600 dark:text-slate-400">
      {{ article.summary }}
    </p>
    <div class="mt-auto flex flex-wrap items-center gap-2 pt-6 text-xs">
      <NuxtLink v-for="tag in article.tags" :key="tag" :to="`/tags/${encodeURIComponent(tag)}`" class="max-w-full break-words rounded-full bg-[#e7eee9] px-2 py-1 text-[#365f53] hover:underline dark:bg-emerald-950 dark:text-emerald-200">
        {{ tag }}
      </NuxtLink>
    </div>
  </article>
</template>
