<script setup lang="ts">
// Comment moderation: pending comments become publicly visible only through
// the approve action here; deletion is permanent. Mirrors the posts table.
import { h, onMounted, ref, watch } from 'vue'
import { NButton, NDataTable, NPopconfirm, NSelect, NTag, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'

import { formatDate } from '~/composables/useArticles'
import { COMMENT_STATUS, COMMENT_STATUS_LABEL, type Comment, type CommentSet } from '~/composables/useComments'

definePageMeta({ layout: 'admin', middleware: 'admin-auth' })

useHead({ title: '评论管理' })

const { authFetch } = useAuth()
const message = useMessage()

const comments = ref<Comment[]>([])
const loading = ref(false)
// Filter literals match the AIP filter contract: status is an int literal.
const statusFilter = ref<string | null>(null)

const filterOptions = [
  { label: '全部状态', value: 'ALL' },
  { label: '待审核', value: '1' },
  { label: '已通过', value: '2' },
]

async function load() {
  loading.value = true
  try {
    const query: Record<string, unknown> = { page_size: 50 }
    if (statusFilter.value && statusFilter.value !== 'ALL')
      query.filter = `status = ${statusFilter.value}`
    const set = await authFetch<CommentSet>('/api/v1/comments', { query })
    comments.value = set.comments ?? []
  }
  catch {
    message.error('评论列表加载失败')
  }
  finally {
    loading.value = false
  }
}

onMounted(load)
watch(statusFilter, load)

async function approve(comment: Comment) {
  try {
    await authFetch(`/api/v1/comments/${comment.id}/approve`, { method: 'POST', headers: { 'Content-Type': 'application/json' } })
    message.success('已通过')
  }
  catch {
    message.error('操作失败')
  }
  load()
}

async function removeComment(comment: Comment) {
  try {
    await authFetch(`/api/v1/comments/${comment.id}`, { method: 'DELETE' })
    message.success('已删除')
  }
  catch {
    message.error('删除失败')
  }
  load()
}

const columns: DataTableColumns<Comment> = [
  {
    title: '文章',
    key: 'article_slug',
    width: 160,
    render: row => h('code', { class: 'text-xs text-slate-500 dark:text-slate-400' }, { default: () => row.article_slug }),
  },
  {
    title: '昵称',
    key: 'display_name',
    width: 120,
    render: row => h('span', { class: 'font-medium' }, { default: () => row.display_name }),
  },
  {
    title: '内容',
    key: 'content',
    ellipsis: { tooltip: true },
    render: row => h('span', {}, { default: () => row.content }),
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render: (row) => {
      const type = row.status === COMMENT_STATUS.APPROVED ? 'success' : 'warning'
      return h(NTag, { type, size: 'small', bordered: false }, { default: () => COMMENT_STATUS_LABEL[row.status] })
    },
  },
  {
    title: '时间',
    key: 'created_at',
    width: 120,
    render: row => formatDate(row.created_at),
  },
  {
    title: '操作',
    key: 'actions',
    width: 160,
    render: (row) => {
      const buttons: ReturnType<typeof h>[] = []
      if (row.status !== COMMENT_STATUS.APPROVED) {
        buttons.push(h(NButton, {
          size: 'tiny',
          quaternary: true,
          type: 'success',
          onClick: () => approve(row),
        }, { default: () => '通过' }))
      }
      buttons.push(h(NPopconfirm, {
        onPositiveClick: () => removeComment(row),
      }, {
        trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, { default: () => '删除' }),
        default: () => '确定删除这条评论吗？',
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
        评论管理
      </h1>
      <NSelect v-model:value="statusFilter" :options="filterOptions" class="w-32" size="small" placeholder="全部状态" />
    </div>

    <NDataTable
      :columns="columns"
      :data="comments"
      :loading="loading"
      :bordered="false"
      size="small"
      :row-key="(row: Comment) => row.id"
    />
  </div>
</template>
