<script setup lang="ts">
usePageSeo({ title: '文章', description: '从实际项目出发，浏览 Python 后端、全栈开发、AIGC 与 AI Agent 的实现、取舍和复盘。' })
</script>

<template>
  <div class="space-y-5">
    <ArticleCollection inline-search>
      <template #heading>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">文章</h1>
      </template>
      <template #actions>
        <NuxtLink to="/collections" class="inline-flex min-h-11 items-center gap-1.5 whitespace-nowrap rounded-full px-3 text-sm text-[var(--text-muted)] transition-colors hover:bg-[var(--surface-hover)] hover:text-[var(--accent)]">
          <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" class="size-4">
            <path d="M6 4.5h12v16l-6-4-6 4z" stroke-linejoin="round" />
          </svg>
          我的收藏
        </NuxtLink>
      </template>
    </ArticleCollection>
  </div>
</template>
