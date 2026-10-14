<script setup lang="ts">
// Article management list. Admin-token list calls see every lifecycle state
// (public lists return published articles only), so drafts appear here.
// Status transitions go through UpdateArticle with a status-only field mask;
// removal is the soft DeleteArticle.
import { NButton, NDataTable, NPopconfirm, NSelect, NTag, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'

import { ARTICLE_STATUS, ARTICLE_STATUS_LABEL, type Article, type ArticleSet, formatDate } from '~/composables/useArticles'

definePageMeta({ layout: 'admin', middleware: 'admin-auth' })

useHead({ title: '文章管理' })

const { authFetch } = useAuth()
const message = useMessage()

const articles = ref<Article[]>([])
const loading = ref(false)
const statusFilter = ref<string | null>(null)
const nextPageToken = ref('')
const pageTokens = ref([''])
const pageIndex = ref(0)
const busySlugs = ref(new Set<string>())
const failedRequest = ref<{ reset: boolean, targetIndex: number, targetTokens: string[] } | null>(null)
let loadVersion = 0

const filterOptions = [
  { label: '全部状态', value: 'ALL' },
  { label: '草稿', value: 'DRAFT' },
  { label: '已发布', value: 'PUBLISHED' },
]

async function load(reset = false, targetIndex = pageIndex.value, targetTokens = pageTokens.value) {
  const version = ++loadVersion
  if (reset) {
    targetTokens = ['']
    targetIndex = 0
    pageTokens.value = targetTokens
    pageIndex.value = targetIndex
    nextPageToken.value = ''
    articles.value = []
  }
  failedRequest.value = null
  loading.value = true
  try {
    const query: Record<string, unknown> = {
      page_size: 50,
      page_token: targetTokens[targetIndex] || undefined,
      order_by: 'updated_at desc,slug',
    }
    if (statusFilter.value && statusFilter.value !== 'ALL')
      query.filter = `status:"${statusFilter.value}"`
    const set = await authFetch<ArticleSet>('/api/v1/articles/list', { query })
    if (version !== loadVersion)
      return
    articles.value = set.articles ?? []
    pageIndex.value = targetIndex
    pageTokens.value = targetTokens
    nextPageToken.value = set.next_page_token ?? ''
  }
  catch {
    if (version === loadVersion) {
      failedRequest.value = { reset, targetIndex, targetTokens }
      message.error('文章列表加载失败')
    }
  }
  finally {
    if (version === loadVersion)
      loading.value = false
  }
}

function nextPage() {
  if (loading.value || busySlugs.value.size || !nextPageToken.value)
    return
  const tokens = [...pageTokens.value.slice(0, pageIndex.value + 1), nextPageToken.value]
  load(false, pageIndex.value + 1, tokens)
}

function previousPage() {
  if (loading.value || busySlugs.value.size || pageIndex.value === 0)
    return
  load(false, pageIndex.value - 1)
}

function retryLoad() {
  const request = failedRequest.value
  if (!request || loading.value)
    return
  load(request.reset, request.targetIndex, request.targetTokens)
}

onMounted(() => load(true))
watch(statusFilter, () => load(true))

async function setStatus(article: Article, status: 1 | 2) {
  if (busySlugs.value.has(article.slug))
    return
  busySlugs.value.add(article.slug)
  try {
    // The proto marks title/content_md REQUIRED, so every update carries the
    // full article and the status-only mask decides what is applied.
    await authFetch('/api/v1/articles/update', {
      method: 'PUT',
      query: { update_mask: 'status' },
      body: {
        slug: article.slug,
        title: article.title,
        summary: article.summary,
        content_md: article.content_md,
        tags: article.tags,
        status,
      },
    })
    message.success(status === ARTICLE_STATUS.PUBLISHED ? '已发布' : '已转为草稿')
    await load(true)
  }
  catch {
    message.error('状态更新失败')
  }
  finally {
    busySlugs.value.delete(article.slug)
  }
}

async function removeArticle(article: Article) {
  if (busySlugs.value.has(article.slug))
    return
  busySlugs.value.add(article.slug)
  try {
    await authFetch(`/api/v1/articles/${encodeURIComponent(article.slug)}`, { method: 'DELETE' })
    message.success('已删除')
    await load(true)
  }
  catch {
    message.error('删除失败')
  }
  finally {
    busySlugs.value.delete(article.slug)
  }
}

const columns: DataTableColumns<Article> = [
  {
    title: '标题',
    key: 'title',
    render: row => h(
      'span',
      { class: 'font-medium' },
      { default: () => row.title },
    ),
  },
  {
    title: 'Slug',
    key: 'slug',
    render: row => h('code', { class: 'text-xs text-slate-500 dark:text-slate-400' }, { default: () => row.slug }),
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render: (row) => {
      const type = row.status === ARTICLE_STATUS.PUBLISHED ? 'success' : 'warning'
      return h(NTag, { type, size: 'small', bordered: false }, { default: () => ARTICLE_STATUS_LABEL[row.status] })
    },
  },
  {
    title: '标签',
    key: 'tags',
    render: row => h(
      'span',
      { class: 'text-xs text-slate-500 dark:text-slate-400' },
      { default: () => (row.tags ?? []).join(' / ') },
    ),
  },
  {
    title: '更新时间',
    key: 'updated_at',
    width: 120,
    render: row => formatDate(row.updated_at),
  },
  {
    title: '操作',
    key: 'actions',
    width: 250,
    render: (row) => {
      const buttons = [
        h(NButton, {
          size: 'tiny',
          quaternary: true,
          type: 'primary',
          tag: 'a',
          href: `/admin/posts/${encodeURIComponent(row.slug)}/edit`,
          onClick: (e: MouseEvent) => {
            e.preventDefault()
            navigateTo(`/admin/posts/${encodeURIComponent(row.slug)}/edit`)
          },
        }, { default: () => '编辑' }),
      ]
      if (row.status !== ARTICLE_STATUS.PUBLISHED) {
        buttons.push(h(NButton, {
          size: 'tiny',
          quaternary: true,
          type: 'success',
          disabled: busySlugs.value.has(row.slug),
          onClick: () => setStatus(row, ARTICLE_STATUS.PUBLISHED),
        }, { default: () => '发布' }))
      }
      else {
        buttons.push(h(NButton, {
          size: 'tiny',
          quaternary: true,
          disabled: busySlugs.value.has(row.slug),
          onClick: () => setStatus(row, ARTICLE_STATUS.DRAFT),
        }, { default: () => '转为草稿' }))
      }
      buttons.push(h(NPopconfirm, {
        disabled: busySlugs.value.has(row.slug),
        onPositiveClick: () => removeArticle(row),
      }, {
        trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error', disabled: busySlugs.value.has(row.slug) }, { default: () => '删除' }),
        default: () => '确定删除这篇文章吗？',
      }))
      return h('div', { class: 'flex items-center gap-1' }, { default: () => buttons })
    },
  },
]
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-xl font-semibold">
        文章管理
      </h1>
      <div class="flex items-center gap-3">
        <NSelect v-model:value="statusFilter" :options="filterOptions" class="w-32 shrink-0" size="small" placeholder="全部状态" />
        <NButton type="primary" tag="a" href="/admin/posts/new" @click.prevent="navigateTo('/admin/posts/new')">
          新建文章
        </NButton>
      </div>
    </div>

    <NDataTable
      :columns="columns"
      :data="articles"
      :loading="loading"
      :bordered="false"
      size="small"
      :row-key="(row: Article) => row.slug"
    />

    <div v-if="failedRequest" class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300" role="alert">
      <span>文章列表暂时无法加载。</span>
      <NButton size="small" type="error" secondary :loading="loading" @click="retryLoad">
        重新加载
      </NButton>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-3 text-sm text-slate-500 dark:text-slate-400">
      <span>第 {{ pageIndex + 1 }} 页 · 本页 {{ articles.length }} 条</span>
      <div class="flex items-center gap-2">
        <NButton size="small" :disabled="loading || busySlugs.size > 0 || pageIndex === 0" @click="previousPage">
          上一页
        </NButton>
        <NButton size="small" :disabled="loading || busySlugs.size > 0 || !nextPageToken" @click="nextPage">
          下一页
        </NButton>
      </div>
    </div>
  </div>
</template>
