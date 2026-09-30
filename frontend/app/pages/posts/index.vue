<script setup lang="ts">
useHead({ title: '文章' })
</script>

<template>
  <div class="space-y-9">
    <header class="space-y-4 py-5">
      <p class="text-xs text-[#3c5d85] dark:text-blue-300">Writing / 技术记录</p>
      <h1 class="text-3xl font-medium text-slate-800 dark:text-slate-100">文章</h1>
      <p class="max-w-xl text-sm leading-8 text-slate-600 dark:text-slate-400">从实际项目出发，记录 Go、全栈开发与 AI Agent 的实现、取舍和复盘。</p>
    </header>
    <ArticleCollection />
  </div>
</template>
