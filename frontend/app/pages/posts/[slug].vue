<script setup lang="ts">
import { ARTICLE_STATUS_LABEL, formatDate, useArticleBySlug } from '~/composables/useArticles'

const route = useRoute()
const slug = computed(() => String(route.params.slug))

const { data, error } = await useArticleBySlug(slug.value)

if (error.value || !data.value) {
  throw createError({ statusCode: 404, message: `文章不存在：${slug.value}` })
}

// Non-null by the 404 guard above; the backend renderer owns the HTML.
const article = computed(() => data.value!)

useHead({ title: article.value.title })
</script>

<template>
  <article class="space-y-6">
    <header class="space-y-3 border-b border-slate-200 pb-6">
      <h1 class="text-3xl font-bold tracking-tight">{{ article.title }}</h1>
      <div class="flex items-center gap-3 text-sm text-slate-500">
        <time>{{ formatDate(article.published_at || article.created_at) }}</time>
        <span
          v-if="article.status !== 2"
          class="rounded bg-amber-100 px-1.5 py-0.5 text-xs text-amber-700"
        >{{ ARTICLE_STATUS_LABEL[article.status] }}</span>
        <span v-for="tag in article.tags" :key="tag" class="rounded bg-slate-100 px-1.5 py-0.5">
          {{ tag }}
        </span>
      </div>
    </header>

    <!-- eslint-disable-next-line vue/no-v-html -- trusted backend-rendered HTML -->
    <div class="markdown-body space-y-4 leading-7" v-html="article.content_html" />
  </article>
</template>
