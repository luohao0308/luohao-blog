<script setup lang="ts">
import { usePublicCategories } from '~/composables/useCategories'

const route = useRoute()
const slug = computed(() => String(route.params.slug))
const { data: categories } = await usePublicCategories()
const category = computed(() => (categories.value ?? []).find(item => item.slug === slug.value))
useHead(() => ({ title: `${category.value?.name ?? slug.value} · 分类` }))
</script>

<template>
  <div class="space-y-9">
    <header class="space-y-4 py-5">
      <NuxtLink to="/categories" class="text-xs text-[#3c5d85] hover:underline dark:text-blue-300">返回全部分类</NuxtLink>
      <h1 class="break-words text-3xl font-medium text-slate-800 dark:text-slate-100">{{ category?.name ?? slug }}</h1>
      <p class="text-sm text-slate-500 dark:text-slate-400">按分类浏览技术记录。</p>
    </header>
    <ArticleCollection :category-slug="slug" />
  </div>
</template>
