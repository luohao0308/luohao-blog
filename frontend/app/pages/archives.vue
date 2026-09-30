<script setup lang="ts">
import { ARTICLE_STATUS, articleDate, formatDate, useArticleList } from '~/composables/useArticles'

const { data, error } = useArticleList({ pageSize: 50 })

useHead({ title: '归档' })

// groupByYear archives articles under their publish (or create) year.
const grouped = computed(() => {
  const list = [...(data.value?.articles ?? [])].sort((a, b) => {
    const ta = articleDate(a.published_at || a.created_at)
    const tb = articleDate(b.published_at || b.created_at)
    return tb.localeCompare(ta)
  })
  const byYear = new Map<string, typeof list>()
  for (const a of list) {
    const year = articleDate(a.published_at || a.created_at).slice(0, 4)
    const bucket = byYear.get(year) ?? []
    bucket.push(a)
    byYear.set(year, bucket)
  }
  return [...byYear.entries()].sort(([y1], [y2]) => y2.localeCompare(y1))
})

function isDraft(status: number): boolean {
  return status === ARTICLE_STATUS.DRAFT
}
</script>

<template>
  <section class="space-y-8">
    <h1 class="text-2xl font-bold tracking-tight">归档</h1>

    <div v-if="error" class="text-red-600">加载失败：{{ error.message }}</div>
    <div v-else-if="!data?.articles?.length" class="text-slate-500">暂无文章。</div>

    <div v-for="[year, articles] in grouped" :key="year" class="space-y-3">
      <h2 class="text-xl font-semibold text-slate-800">{{ year }}</h2>
      <ul class="space-y-2">
        <li v-for="a in articles" :key="a.id" class="flex items-baseline justify-between gap-3 border-b border-dashed border-slate-200 pb-2">
          <NuxtLink :to="`/posts/${a.slug}`" class="text-slate-700 hover:text-slate-900">
            {{ a.title }}
            <span
              v-if="isDraft(a.status)"
              class="ml-2 rounded bg-amber-100 px-1.5 py-0.5 text-xs text-amber-700"
            >草稿</span>
          </NuxtLink>
          <time class="shrink-0 text-xs text-slate-500">{{ formatDate(a.published_at || a.created_at) }}</time>
        </li>
      </ul>
    </div>
  </section>
</template>
