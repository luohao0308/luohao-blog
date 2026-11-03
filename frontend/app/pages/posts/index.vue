<script setup lang="ts">
usePageSeo({ title: '文章', description: 'Python、Go、全栈开发和 AI 应用相关文章。' })
</script>

<template>
  <div class="space-y-5">
    <ArticleCollection inline-search>
      <template #heading>
        <h1 class="browse-title">文章</h1>
      </template>
      <template #actions>
        <NuxtLink to="/series" class="detail-button detail-button-primary">系列阅读 →</NuxtLink>
        <NuxtLink to="/collections" class="inline-flex min-h-11 items-center gap-1.5 whitespace-nowrap rounded-full px-3 text-sm text-[var(--text-muted)] transition-colors hover:bg-[var(--surface-hover)] hover:text-[var(--accent)]">
          <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" class="size-4">
            <path d="M6 4.5h12v16l-6-4-6 4z" stroke-linejoin="round" />
          </svg>
          我的收藏
        </NuxtLink>
      </template>
      <template #intro="{ series }">
        <nav v-if="series.length" class="series-reading-prompt" aria-label="系列阅读入口">
          <NuxtLink v-for="item in series" :key="item.slug" :to="`/series/${item.slug}`">{{ item.title }} · {{ item.count }} 篇</NuxtLink>
        </nav>
      </template>
    </ArticleCollection>
  </div>
</template>
