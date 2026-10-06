<script setup lang="ts">
import { usePublicCategories } from '~/composables/useCategories'

useHead({ title: '分类' })

const { data, status, error, refresh } = await usePublicCategories()
const categories = computed(() =>
  [...(data.value ?? [])].sort((a, b) => a.sort - b.sort || a.slug.localeCompare(b.slug)),
)
</script>

<template>
  <div class="space-y-8">
    <PageHeader title="分类" description="按分类浏览技术记录，数字是收录的文章数。" />
    <div v-if="error" role="alert" class="space-y-3 py-12 text-center">
      <p class="text-sm text-slate-600 dark:text-slate-400">分类加载失败，请稍后重试。</p>
      <button type="button" class="text-sm text-[#3c5d85] underline dark:text-blue-300" @click="refresh()">重新加载</button>
    </div>
    <p v-else-if="status === 'pending'" role="status" class="py-12 text-center text-sm text-slate-500">正在加载分类…</p>
    <nav v-else-if="categories.length" aria-label="全部分类" class="grid gap-4 sm:grid-cols-2">
      <NuxtLink
        v-for="item in categories"
        :key="item.slug"
        :to="`/categories/${encodeURIComponent(item.slug)}`"
        class="rounded-xl border border-slate-200 p-4 transition-colors hover:border-[#3c5d85] dark:border-slate-800 dark:hover:border-blue-300"
      >
        <div class="flex items-baseline justify-between">
          <span class="font-medium text-slate-800 dark:text-slate-100">{{ item.name }}</span>
          <span class="text-xs text-slate-400 dark:text-slate-500">{{ item.article_count ?? 0 }} 篇</span>
        </div>
        <code class="mt-1 block text-xs text-slate-400 dark:text-slate-500">{{ item.slug }}</code>
      </NuxtLink>
    </nav>
    <p v-else class="py-16 text-center text-sm text-slate-500 dark:text-slate-400">后台建立分类后，它们会集中在这里。</p>
  </div>
</template>
