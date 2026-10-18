<script setup lang="ts">
import { articleDate } from '~/composables/useArticles'

useHead({ title: '收藏' })
const store = useCollections()
const { guest, cloud, owner, ready, loading, error, busy } = store
const visibleCount = ref(20)
const importPending = ref(false)
const notice = ref('')
const items = computed(() => owner.value
  ? cloud.value.map(row => ({ slug: row.article.slug, title: row.article.title, saved_at: articleDate(row.saved_at) }))
  : guest.value)
const visible = computed(() => items.value.slice(0, visibleCount.value))

onMounted(() => store.initialize(true))

async function remove(slug: string, title: string) {
  notice.value = ''
  try { await store.toggle(slug, title) }
  catch { notice.value = '移除失败，请重试。' }
}

async function importLocal() {
  if (importPending.value) return
  importPending.value = true
  notice.value = ''
  const result = await store.importGuest()
  notice.value = result.failed
    ? `已导入 ${result.imported} 条，${result.failed} 条失败并保留在本地，可稍后重试。`
    : `已导入 ${result.imported} 条本地收藏。`
  importPending.value = false
}
</script>

<template>
  <div class="space-y-8">
    <PageHeader title="收藏" :description="owner ? '收藏已与当前登录账号同步。' : '未登录时收藏保存在当前浏览器；登录后可显式导入云端。'" />
    <div v-if="owner" class="flex justify-end"><button type="button" :disabled="loading" class="text-sm text-blue-600 hover:underline disabled:opacity-50" @click="store.loadCloud(true)">刷新云端</button></div>
    <div v-if="owner && guest.length" class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-amber-200 bg-amber-50 p-4 dark:border-amber-900 dark:bg-amber-950/30">
      <p class="text-sm text-amber-800 dark:text-amber-200">当前浏览器还有 {{ guest.length }} 条本地收藏，不会自动导入。</p>
      <button type="button" :disabled="!ready || importPending" class="rounded-full bg-amber-700 px-4 py-2 text-sm text-white disabled:opacity-50" @click="importLocal">{{ importPending ? '导入中…' : '导入本地收藏' }}</button>
    </div>
    <p v-if="notice" role="status" class="text-sm text-slate-600 dark:text-slate-300">{{ notice }}</p>
    <div v-if="error" role="alert" class="space-y-2 text-center"><p>{{ error }}</p><button class="text-blue-600 underline" @click="store.loadCloud(true)">重新加载</button></div>
    <p v-else-if="!ready" role="status" class="py-12 text-center text-sm text-slate-500">正在加载收藏…</p>
    <ul v-else-if="visible.length" class="space-y-3">
      <li v-for="entry in visible" :key="entry.slug" class="bento-card flex items-center justify-between gap-4 rounded-xl border border-slate-200 p-4 dark:border-slate-800">
        <div class="min-w-0"><NuxtLink :to="`/posts/${encodeURIComponent(entry.slug)}`" class="block truncate font-medium hover:text-blue-600">{{ entry.title }}</NuxtLink><p class="mt-1 text-xs text-slate-400">收藏于 {{ new Date(entry.saved_at).toLocaleDateString('zh-CN') }}</p></div>
        <button type="button" :disabled="!ready || busy.has(entry.slug)" class="shrink-0 rounded-full px-3 py-1.5 text-xs text-slate-500 hover:text-rose-600 disabled:opacity-50" @click="remove(entry.slug, entry.title)">移除</button>
      </li>
    </ul>
    <p v-else class="py-16 text-center text-sm text-slate-500">还没有收藏。在文章页点 ☆ 收藏，它就会出现在这里。</p>
    <button v-if="visibleCount < items.length" type="button" class="mx-auto block rounded-full border border-slate-300 px-4 py-2 text-sm" @click="visibleCount += 20">加载更多</button>
  </div>
</template>
