<script setup lang="ts">
import { articleDate, formatDate, type Article } from '~/composables/useArticles'
import { articleSummary, readingMinutes } from '~/utils/article'

const props = defineProps<{
  article: Article
}>()

const openArticle = useArticleExpansion()
const articleTo = computed(() => `/posts/${encodeURIComponent(props.article.slug)}`)

const summary = computed(() => articleSummary(props.article.summary, props.article.content_md))

</script>

<template>
  <article data-article-card class="bento-card cursor-pointer flex h-full min-w-0 flex-col overflow-hidden rounded-[var(--radius-panel)] border border-[var(--border-default)] bg-[var(--surface-page)] transition-colors hover:border-[var(--border-strong)]" @click.capture="openArticle($event, articleTo)">
    <NuxtLink v-if="article.cover_url" :to="`/posts/${encodeURIComponent(article.slug)}`" class="block overflow-hidden bg-[var(--surface-panel)]">
      <img :src="assetUrl(article.cover_url)" :alt="`${article.title}封面`" class="aspect-[16/9] w-full object-cover transition-transform duration-300 hover:scale-[1.02]">
    </NuxtLink>
    <div class="flex flex-1 flex-col p-6">
      <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-[var(--text-muted)] tabular-nums">
        <time :datetime="articleDate(article.published_at || article.created_at)">{{ formatDate(article.published_at || article.created_at) }}</time>
        <span>{{ readingMinutes(article.content_md) }} 分钟阅读</span>
      </div>
      <span v-if="article.featured" class="mt-3 w-fit rounded-full bg-[var(--status-warning-surface)] px-2 py-0.5 text-[11px] text-[var(--status-warning)]">精选</span>
      <h2 class="mt-3 text-xl font-medium leading-relaxed text-[var(--text-primary)]">
        <NuxtLink :to="`/posts/${encodeURIComponent(article.slug)}`" class="break-words hover:text-[var(--accent)]">
          {{ article.title }}
        </NuxtLink>
      </h2>
      <p v-if="summary" class="mt-3 line-clamp-3 text-sm leading-7 text-[var(--text-muted)]">
        {{ summary }}
      </p>
      <div class="mt-auto flex flex-wrap items-center gap-2 pt-6 text-xs">
        <NuxtLink v-for="tag in article.tags" :key="tag" :to="`/tags/${encodeURIComponent(tag)}`" class="max-w-full break-words rounded-full bg-[var(--mint-surface)] px-2 py-1 text-[var(--mint-text)] hover:underline">
          {{ tag }}
        </NuxtLink>
      </div>
    </div>
  </article>
</template>
