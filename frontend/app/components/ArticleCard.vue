<script setup lang="ts">
import { ARTICLE_STATUS, ARTICLE_STATUS_LABEL, formatDate, type Article } from '~/composables/useArticles'

defineProps<{
  article: Article
}>()

function statusLabel(a: Article): string {
  return ARTICLE_STATUS_LABEL[a.status] ?? '未知'
}

function isDraft(a: Article): boolean {
  return a.status === ARTICLE_STATUS.DRAFT
}
</script>

<template>
  <NuxtLink
    :to="`/posts/${article.slug}`"
    class="block rounded-lg border border-slate-200 p-5 transition-shadow hover:shadow-md"
  >
    <div class="flex items-baseline justify-between gap-3">
      <h3 class="text-lg font-semibold tracking-tight text-slate-900">
        {{ article.title }}
      </h3>
      <span
        v-if="isDraft(article)"
        class="shrink-0 rounded bg-amber-100 px-1.5 py-0.5 text-xs text-amber-700"
      >{{ statusLabel(article) }}</span>
    </div>
    <p v-if="article.summary" class="mt-1 line-clamp-2 text-sm text-slate-600">
      {{ article.summary }}
    </p>
    <div class="mt-3 flex items-center gap-3 text-xs text-slate-500">
      <time>{{ formatDate(article.published_at || article.created_at) }}</time>
      <span v-for="tag in article.tags" :key="tag" class="rounded bg-slate-100 px-1.5 py-0.5">
        {{ tag }}
      </span>
    </div>
  </NuxtLink>
</template>
