<script setup lang="ts">
import { formatDate, useArticleList } from '~/composables/useArticles'

const { data, error, status } = useArticleList({ pageSize: 10 })

useHead({ title: '首页' })
</script>

<template>
  <section class="space-y-6">
    <div class="space-y-2">
      <h1 class="text-3xl font-bold tracking-tight">全栈 & AI Agent 的个人博客</h1>
      <p class="text-slate-600">
        记录全栈开发与 AI Agent 的实践、踩坑与思考。
      </p>
    </div>

    <div v-if="status === 'pending'" class="text-slate-500">加载中…</div>
    <div v-else-if="error" class="text-red-600">文章加载失败：{{ error.message }}</div>
    <div v-else-if="!data?.articles?.length" class="text-slate-500">
      还没有文章——第一篇正在路上。
    </div>

    <div v-else class="space-y-4">
      <ArticleCard v-for="a in data.articles" :key="a.id" :article="a" />
    </div>

    <p v-if="data?.articles?.length" class="text-sm text-slate-500">
      最近更新：{{ formatDate(data.articles[0]?.updated_at) }}
    </p>
  </section>
</template>
