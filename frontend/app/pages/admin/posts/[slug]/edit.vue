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

useHead({ title: computed(() => (article.value ? `编辑：${article.value.title}` : '编辑文章')) })

// Reload on a reused /:slug/edit route and ignore stale reads.
let loadVersion = 0
async function loadArticle() {
  const version = ++loadVersion
  article.value = null
  loadFailed.value = false
  saveState.value = 'idle'
  try {
    const loaded = await authFetch<Article>(`/api/v1/articles/${encodeURIComponent(slug.value)}`)
    if (version === loadVersion)
      article.value = loaded
  }
  catch {
    if (version === loadVersion)
      loadFailed.value = true
  }
}
onMounted(loadArticle)
watch(slug, loadArticle)

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

async function onSubmit(payload: FormPayload) {
  if (submitting.value)
    return
  submitting.value = true
  saveState.value = 'saving'
  const owner = formRef.value
  try {
    // slug stays read-only in the form and out of the mask: it only
    // identifies the record here. A cleared select sends an empty category
    // brief, which the server stores as uncategorized.
    await authFetch<Article>('/api/v1/articles/update', {
      method: 'PUT',
      query: { update_mask: 'title,summary,content_md,tags,category,status,cover_url,featured' },
      body: {
        ...payload,
        slug: slug.value,
        category: payload.category_slug ? { slug: payload.category_slug } : {},
      },
    })
    if (disposed || owner !== formRef.value)
      return
    owner?.markSaved(payload)
    saveState.value = 'saved'
    message.success('已保存')
  }
  catch {
    if (disposed || owner !== formRef.value)
      return
    saveState.value = 'error'
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
      ref="formRef"
      :key="article.slug"
      :initial="article"
      :submitting="submitting"
      @submit="onSubmit"
    >
      <template #actions>
        <p data-testid="save-state" role="status" aria-live="polite" class="text-sm">{{ saveLabel }}</p>
        <NButton type="primary" attr-type="submit" :loading="submitting">
          保存
        </NButton>
      </template>
    </AdminArticleForm>
  </div>
</template>
