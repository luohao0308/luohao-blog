<script setup lang="ts">
useHead({ title: '文章' })
</script>

<template>
  <div class="space-y-8">
    <PageHeader title="文章" description="从实际项目出发，记录 Go、全栈开发与 AI Agent 的实现、取舍和复盘。" />
    <!-- taxonomy entries used to live in the header 文章 dropdown; they are
         now surfaced here so the top nav can stay flat. -->
    <nav aria-label="内容入口" class="flex flex-wrap items-center gap-2 text-sm">
      <span class="mr-1 text-slate-400 dark:text-slate-500">浏览</span>
      <NuxtLink
        to="/categories"
        class="rounded-full border border-slate-200 px-3.5 py-1.5 text-slate-600 transition-colors hover:border-blue-400 hover:text-blue-600 dark:border-slate-700 dark:text-slate-300 dark:hover:border-blue-400 dark:hover:text-blue-300"
      >分类</NuxtLink>
      <NuxtLink
        to="/tags"
        class="rounded-full border border-slate-200 px-3.5 py-1.5 text-slate-600 transition-colors hover:border-blue-400 hover:text-blue-600 dark:border-slate-700 dark:text-slate-300 dark:hover:border-blue-400 dark:hover:text-blue-300"
      >标签</NuxtLink>
      <NuxtLink
        to="/archives"
        class="rounded-full border border-slate-200 px-3.5 py-1.5 text-slate-600 transition-colors hover:border-blue-400 hover:text-blue-600 dark:border-slate-700 dark:text-slate-300 dark:hover:border-blue-400 dark:hover:text-blue-300"
      >归档</NuxtLink>
    </nav>
    <ArticleCollection />
  </div>
</template>
