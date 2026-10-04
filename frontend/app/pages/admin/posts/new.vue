<script setup lang="ts">
// New article page. CreateArticle always starts as DRAFT server-side; the
// status radio is sent in the create payload and the usecase applies the
// published transition when PUBLISHED is chosen.
import type { Article } from '~/composables/useArticles'

import { NButton, useMessage } from 'naive-ui'

definePageMeta({ layout: 'admin', middleware: 'admin-auth' })

useHead({ title: '新建文章' })

const { authFetch } = useAuth()
const message = useMessage()

const submitting = ref(false)

interface FormPayload {
  title: string
  slug: string
  summary: string
  content_md: string
  tags: string[]
  category_slug: string | null
  status: number
}

// A chosen category rides as a CategoryBrief; an empty brief means
// uncategorized, which the backend stores as a NULL foreign key.
function categoryOf(payload: FormPayload) {
  return payload.category_slug ? { slug: payload.category_slug } : {}
}

async function onSubmit(payload: FormPayload) {
  submitting.value = true
  try {
    const article = await authFetch<Article>('/api/v1/articles/create', {
      method: 'POST',
      body: { ...payload, category: categoryOf(payload) },
    })
    message.success('已创建')
    navigateTo(`/admin/posts/${encodeURIComponent(article.slug)}/edit`)
  }
  catch {
    message.error('创建失败，请检查 slug 是否重复')
  }
  finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-semibold">
        新建文章
      </h1>
      <NuxtLink to="/admin/posts" class="text-sm text-slate-500 hover:underline dark:text-slate-400">
        返回列表
      </NuxtLink>
    </div>

    <AdminArticleForm :submitting="submitting" @submit="onSubmit">
      <template #actions>
        <NButton type="primary" attr-type="submit" :loading="submitting">
          创建
        </NButton>
      </template>
    </AdminArticleForm>
  </div>
</template>
