<script setup lang="ts">
usePageSeo({ title: '文章', description: '从实际项目出发，浏览 Go、全栈开发与 AI Agent 的实现、取舍和复盘。' })
</script>

<template>
  <div class="space-y-5">
    <ArticleCollection inline-search>
      <template #heading>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">文章</h1>
      </template>
    </ArticleCollection>
  </div>
</template>
