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

let disposed = false
onBeforeUnmount(() => { disposed = true })
const submitting = ref(false)
const formRef = ref<{ markSaved: (payload: FormPayload) => void, dirty: boolean } | null>(null)
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const saveLabel = computed(() => {
  if (saveState.value === 'saving') return '保存中…'
  if (saveState.value === 'error') return '保存失败，修改已保留，请重试'
  if (formRef.value?.dirty) return '有未保存的修改'
  return saveState.value === 'saved' ? '已保存' : '尚无修改'
})

interface FormPayload {
  title: string
  slug: string
  summary: string
  content_md: string
  tags: string[]
  category_slug: string | null
  status: number
  cover_url: string
  featured: boolean
}

// A chosen category rides as a CategoryBrief; an empty brief means
// uncategorized, which the backend stores as a NULL foreign key.
function categoryOf(payload: FormPayload) {
  return payload.category_slug ? { slug: payload.category_slug } : {}
}

async function onSubmit(payload: FormPayload) {
  if (submitting.value)
    return
  submitting.value = true
  saveState.value = 'saving'
  const owner = formRef.value
  try {
    const article = await authFetch<Article>('/api/v1/articles/create', {
      method: 'POST',
      body: { ...payload, category: categoryOf(payload) },
    })
    if (disposed || owner !== formRef.value)
      return
    owner?.markSaved(payload)
    saveState.value = 'saved'
    message.success('已创建')
    await navigateTo(`/admin/posts/${encodeURIComponent(article.slug)}/edit`)
  }
  catch {
    if (disposed || owner !== formRef.value)
      return
    saveState.value = 'error'
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

    <AdminArticleForm ref="formRef" :frozen="submitting" :submitting="submitting" @submit="onSubmit">
      <template #actions>
        <p data-testid="save-state" role="status" aria-live="polite" class="text-sm">{{ saveLabel }}</p>
        <NButton type="primary" attr-type="submit" :loading="submitting">
          创建
        </NButton>
      </template>
    </AdminArticleForm>
  </div>
</template>
