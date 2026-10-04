<script setup lang="ts">
import { tagCounts, usePublishedArticles } from '~/composables/useArticles'

useHead({ title: '标签' })

const { data, status, error, refresh } = await usePublishedArticles()
const tags = computed(() => tagCounts(data.value ?? []))
</script>

<template>
  <div class="space-y-9">
    <header class="space-y-4 py-5">
      <p class="text-xs text-[#3c5d85] dark:text-blue-300">Browse / 内容组织</p>
      <h1 class="text-3xl font-medium text-slate-800 dark:text-slate-100">标签</h1>
      <p class="max-w-xl text-sm leading-8 text-slate-600 dark:text-slate-400">按标签浏览技术记录，数字是收录的文章数。</p>
    </header>
    <div v-if="error" role="alert" class="space-y-3 py-12 text-center">
      <p class="text-sm text-slate-600 dark:text-slate-400">标签加载失败，请稍后重试。</p>
      <button type="button" class="text-sm text-[#3c5d85] underline dark:text-blue-300" @click="refresh()">重新加载</button>
    </div>
    <p v-else-if="status === 'pending'" role="status" class="py-12 text-center text-sm text-slate-500">正在加载标签…</p>
    <nav v-else-if="tags.length" aria-label="全部标签" class="flex flex-wrap gap-3">
      <NuxtLink
        v-for="item in tags"
        :key="item.tag"
        :to="`/tags/${encodeURIComponent(item.tag)}`"
        class="rounded-lg border border-slate-200 px-4 py-2 transition-colors hover:border-[#3c5d85] dark:border-slate-800 dark:hover:border-blue-300"
      >
        <span class="text-sm text-slate-700 dark:text-slate-200">{{ item.tag }}</span>
        <span class="ml-2 text-xs text-slate-400 dark:text-slate-500">{{ item.count }}</span>
      </NuxtLink>
    </nav>
    <p v-else class="py-16 text-center text-sm text-slate-500 dark:text-slate-400">文章发布后标签会集中在这里。</p>
  </div>
</template>
