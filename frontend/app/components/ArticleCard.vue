<script setup lang="ts">
import { articleDate, formatDate, type Article } from '~/composables/useArticles'
import { articleSummary, readingMinutes } from '~/utils/article'

const props = defineProps<{
  article: Article
  coverSrc?: string
  coverAlt?: string
}>()
const destination = computed(() => `/posts/${encodeURIComponent(props.article.slug)}`)
const topic = computed(() => props.article.category?.name || props.article.tags?.[0] || '开发随笔')
const openArticle = useArticleExpansion()
const coverUrl = computed(() => props.coverSrc || assetUrl(props.article.cover_url))
const summary = computed(() => articleSummary(props.article.summary, props.article.content_md))
const failedCover = ref('')
const coverImage = ref<HTMLImageElement | null>(null)
onMounted(() => {
  // An SSR image can fail before hydration attaches the error listener.
  if (coverImage.value?.complete && !coverImage.value.naturalWidth) failedCover.value = coverUrl.value
})
const showCover = computed(() => coverUrl.value && coverUrl.value !== failedCover.value)
</script>

<template>
  <article data-article-card class="editorial-card bento-card reading-card" :class="{ 'has-cover': showCover }" @click.capture="openArticle($event, destination)">
    <div v-if="showCover" class="editorial-cover">
      <img ref="coverImage" :src="coverUrl" :alt="coverAlt || `${article.title}封面`" width="800" height="600" loading="lazy" decoding="async" @error="failedCover = coverUrl">
    </div>
    <div class="editorial-body">
      <div class="editorial-meta">
        <span class="editorial-topic">{{ topic }}</span>
        <span v-if="article.featured" class="editorial-featured">精选</span>
        <time :datetime="articleDate(article.published_at || article.created_at)">{{ formatDate(article.published_at || article.created_at) }}</time>
      </div>
      <div class="editorial-copy">
        <h2 class="editorial-heading">
          <NuxtLink :to="destination" class="editorial-link">{{ article.title }}</NuxtLink>
        </h2>
        <p v-if="summary" class="editorial-summary">{{ summary }}</p>
      </div>
      <div class="editorial-footer">
        <div class="editorial-tags">
          <span class="editorial-reading">{{ readingMinutes(article.content_md) }} 分钟阅读</span>
          <NuxtLink v-for="tag in article.tags?.slice(0, 2)" :key="tag" :to="`/tags/${encodeURIComponent(tag)}`">{{ tag }}</NuxtLink>
          <span v-if="(article.tags?.length || 0) > 2">+{{ article.tags.length - 2 }}</span>
        </div>
        <span class="editorial-arrow" aria-hidden="true">↗</span>
      </div>
    </div>
  </article>
</template>
