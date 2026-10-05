<script setup lang="ts">
useHead({ title: '文章' })
</script>

<template>
  <div class="space-y-8">
    <PageHeader title="文章" description="从实际项目出发，记录 Go、全栈开发与 AI Agent 的实现、取舍和复盘。" />
    <ArticleCollection />
  </div>
</template>
