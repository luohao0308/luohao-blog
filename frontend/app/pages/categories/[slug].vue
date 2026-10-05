<script setup lang="ts">
import { usePublicCategories } from '~/composables/useCategories'

const route = useRoute()
const slug = computed(() => String(route.params.slug))
const { data: categories } = await usePublicCategories()
const category = computed(() => (categories.value ?? []).find(item => item.slug === slug.value))
useHead(() => ({ title: `${category.value?.name ?? slug.value} · 分类` }))
</script>

<template>
  <div class="space-y-8">
    <PageHeader :title="category?.name ?? slug" description="按分类浏览技术记录。">
      <template #eyebrow>
        <NuxtLink to="/categories" class="text-xs text-[#3c5d85] hover:underline dark:text-blue-300">返回全部分类</NuxtLink>
      </template>
    </PageHeader>
    <ArticleCollection :category-slug="slug" />
  </div>
</template>
