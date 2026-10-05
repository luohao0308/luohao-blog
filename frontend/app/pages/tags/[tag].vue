<script setup lang="ts">
const route = useRoute()
const tag = computed(() => String(route.params.tag))
useHead(() => ({ title: `${tag.value} · 文章` }))
</script>

<template>
  <div class="space-y-8">
    <PageHeader :title="tag" description="按标签浏览技术记录。">
      <template #eyebrow>
        <NuxtLink to="/posts" class="text-xs text-[#3c5d85] hover:underline dark:text-blue-300">返回全部文章</NuxtLink>
      </template>
    </PageHeader>
    <ArticleCollection :tag="tag" />
  </div>
</template>
