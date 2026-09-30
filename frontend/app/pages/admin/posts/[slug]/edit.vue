<script setup lang="ts">
// Edit page. Admin-token reads see drafts, so a not-yet-published article
// loads here like any other. Updates send every mutable field with an
// explicit field mask; slug is immutable and stays out of the mask.
import type { Article } from '~/composables/useArticles'

import { NButton, useMessage } from 'naive-ui'

definePageMeta({ layout: 'admin', middleware: 'admin-auth' })

const route = useRoute()
const { authFetch } = useAuth()
const message = useMessage()

const slug = computed(() => String(route.params.slug))
const article = ref<Article | null>(null)
const loadFailed = ref(false)
const submitting = ref(false)

useHead({ title: computed(() => (article.value ? `编辑：${article.value.title}` : '编辑文章')) })

onMounted(async () => {
  try {
    article.value = await authFetch<Article>(`/api/v1/articles/${encodeURIComponent(slug.value)}`)
  }
  catch {
    loadFailed.value = true
  }
})

interface FormPayload {
  title: string
  slug: string
  summary: string
  content_md: string
  tags: string[]
  status: number
}

async function onSubmit(payload: FormPayload) {
  submitting.value = true
  try {
    // slug stays read-only in the form and out of the mask: it only
    // identifies the record here.
    await authFetch<Article>('/api/v1/articles/update', {
      method: 'PUT',
      query: { update_mask: 'title,summary,content_md,tags,status' },
      body: { ...payload, slug: slug.value },
    })
    message.success('已保存')
  }
  catch {
    message.error('保存失败')
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
        编辑文章
      </h1>
      <div class="flex items-center gap-4">
        <NuxtLink :to="`/posts/${encodeURIComponent(slug)}`" class="text-sm text-slate-500 hover:underline dark:text-slate-400">
          查看前台
        </NuxtLink>
        <NuxtLink to="/admin/posts" class="text-sm text-slate-500 hover:underline dark:text-slate-400">
          返回列表
        </NuxtLink>
      </div>
    </div>

    <p v-if="loadFailed" class="text-sm text-red-600 dark:text-red-400">
      文章加载失败，可能不存在或网络异常。
    </p>

    <AdminArticleForm
      v-if="article"
      :key="article.slug"
      :initial="article"
      :submitting="submitting"
      @submit="onSubmit"
    >
      <template #actions>
        <NButton type="primary" attr-type="submit" :loading="submitting">
          保存
        </NButton>
      </template>
    </AdminArticleForm>
  </div>
</template>
