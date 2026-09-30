<script setup lang="ts">
const route = useRoute()
const tag = computed(() => String(route.params.tag))
useHead(() => ({ title: `${tag.value} · 文章` }))
</script>

<template>
  <div class="space-y-9">
    <header class="space-y-4 py-5">
      <NuxtLink to="/posts" class="text-xs text-[#3c5d85] hover:underline dark:text-blue-300">返回全部文章</NuxtLink>
      <h1 class="break-words text-3xl font-medium text-slate-800 dark:text-slate-100">{{ tag }}</h1>
      <p class="text-sm text-slate-500 dark:text-slate-400">按标签浏览技术记录。</p>
    </header>
    <ArticleCollection :tag="tag" />
  </div>
</template>
