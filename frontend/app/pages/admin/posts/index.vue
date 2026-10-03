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

const filterOptions = [
  { label: '全部状态', value: 'ALL' },
  { label: '草稿', value: 'DRAFT' },
  { label: '已发布', value: 'PUBLISHED' },
]

async function load() {
  loading.value = true
  try {
    const query: Record<string, unknown> = { page_size: 50, order_by: 'updated_at desc' }
    if (statusFilter.value && statusFilter.value !== 'ALL')
      query.filter = `status:"${statusFilter.value}"`
    const set = await authFetch<ArticleSet>('/api/v1/articles/list', { query })
    articles.value = set.articles ?? []
  }
  catch {
    message.error('文章列表加载失败')
  }
  finally {
    loading.value = false
  }
}

onMounted(load)
watch(statusFilter, load)

async function setStatus(article: Article, status: 1 | 2) {
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
  }
  catch {
    message.error('状态更新失败')
  }
  load()
}

async function removeArticle(article: Article) {
  try {
    await authFetch(`/api/v1/articles/${encodeURIComponent(article.slug)}`, { method: 'DELETE' })
    message.success('已删除')
  }
  catch {
    message.error('删除失败')
  }
  load()
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
          onClick: () => setStatus(row, ARTICLE_STATUS.PUBLISHED),
        }, { default: () => '发布' }))
      }
      else {
        buttons.push(h(NButton, {
          size: 'tiny',
          quaternary: true,
          onClick: () => setStatus(row, ARTICLE_STATUS.DRAFT),
        }, { default: () => '转为草稿' }))
      }
      buttons.push(h(NPopconfirm, {
        onPositiveClick: () => removeArticle(row),
      }, {
        trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, { default: () => '删除' }),
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
        <NSelect v-model:value="statusFilter" :options="filterOptions" class="w-32" size="small" placeholder="全部状态" />
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
  </div>
</template>
