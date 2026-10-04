<script setup lang="ts">
import { readCollections, writeCollections, type CollectionEntry } from '~/composables/useCollections'

useHead({ title: '收藏' })

// Collections live only in this browser's localStorage, so the list reads on
// mount (client-only) and stays empty during SSR.
const items = ref<CollectionEntry[]>([])
onMounted(() => {
  items.value = readCollections()
})

function remove(entry: CollectionEntry) {
  items.value = items.value.filter(item => item.slug !== entry.slug)
  writeCollections(items.value)
}
</script>

<template>
  <div class="space-y-9">
    <header class="space-y-4 py-5">
      <p class="text-xs text-[#3c5d85] dark:text-blue-300">Browse / 我的收藏</p>
      <h1 class="text-3xl font-medium text-slate-800 dark:text-slate-100">收藏</h1>
      <p class="max-w-xl text-sm leading-8 text-slate-600 dark:text-slate-400">收藏只保存在当前浏览器里，不会同步到服务器；换设备或清理浏览器数据后不会保留。</p>
    </header>

    <ul v-if="items.length" class="space-y-3">
      <li
        v-for="entry in items"
        :key="entry.slug"
        class="flex items-center justify-between gap-4 rounded-lg border border-slate-200 p-4 dark:border-slate-800"
      >
        <div class="min-w-0">
          <NuxtLink
            :to="`/posts/${encodeURIComponent(entry.slug)}`"
            class="block truncate font-medium text-slate-800 hover:text-[#3c5d85] dark:text-slate-100 dark:hover:text-blue-300"
          >{{ entry.title }}</NuxtLink>
          <p class="mt-1 text-xs text-slate-400 dark:text-slate-500">收藏于 {{ new Date(entry.saved_at).toLocaleDateString('zh-CN') }}</p>
        </div>
        <button
          type="button"
          class="shrink-0 rounded px-2 py-1 text-xs text-slate-500 transition-colors hover:bg-rose-50 hover:text-rose-600 dark:hover:bg-rose-950/40 dark:hover:text-rose-300"
          @click="remove(entry)"
        >移除</button>
      </li>
    </ul>
    <p v-else class="py-16 text-center text-sm text-slate-500 dark:text-slate-400">还没有收藏。在文章页点 ☆ 收藏，它就会出现在这里。</p>
  </div>
</template>
