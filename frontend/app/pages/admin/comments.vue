<script setup lang="ts">
// The queue opens on comments held for review; ordinary comments publish automatically.
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
const statusFilter = ref<string | null>('1')
const nextPageToken = ref('')
const pageTokens = ref([''])
const pageIndex = ref(0)
const busyIds = ref(new Set<string>())
let loadVersion = 0

const filterOptions = [
  { label: '全部状态', value: 'ALL' },
  { label: '待审核', value: '1' },
  { label: '已通过', value: '2' },
]

async function load(reset = false, targetIndex = pageIndex.value, targetTokens = pageTokens.value) {
  const version = ++loadVersion
  if (reset) {
    targetTokens = ['']
    targetIndex = 0
    pageTokens.value = targetTokens
    pageIndex.value = targetIndex
    nextPageToken.value = ''
    comments.value = []
  }
  loading.value = true
  try {
    const query: Record<string, unknown> = { page_size: 50, page_token: targetTokens[targetIndex] || undefined }
    if (statusFilter.value && statusFilter.value !== 'ALL')
      query.filter = `status = ${statusFilter.value}`
    const set = await authFetch<CommentSet>('/api/v1/comments', { query })
    if (version !== loadVersion)
      return
    comments.value = set.comments ?? []
    pageIndex.value = targetIndex
    pageTokens.value = targetTokens
    nextPageToken.value = set.next_page_token ?? ''
  }
  catch {
    if (version === loadVersion)
      message.error('评论列表加载失败')
  }
  finally {
    if (version === loadVersion)
      loading.value = false
  }
}

function nextPage() {
  if (loading.value || busyIds.value.size || !nextPageToken.value)
    return
  const tokens = [...pageTokens.value.slice(0, pageIndex.value + 1), nextPageToken.value]
  load(false, pageIndex.value + 1, tokens)
}

function previousPage() {
  if (loading.value || busyIds.value.size || pageIndex.value === 0)
    return
  load(false, pageIndex.value - 1)
}

onMounted(() => load(true))
watch(statusFilter, () => load(true))

async function approve(comment: Comment) {
  if (busyIds.value.has(comment.id))
    return
  busyIds.value.add(comment.id)
  try {
    await authFetch(`/api/v1/comments/${comment.id}/approve`, { method: 'POST', headers: { 'Content-Type': 'application/json' } })
    message.success('已通过')
    await load(true)
  }
  catch {
    message.error('操作失败')
  }
  finally {
    busyIds.value.delete(comment.id)
  }
}

async function removeComment(comment: Comment) {
  if (busyIds.value.has(comment.id))
    return
  busyIds.value.add(comment.id)
  try {
    await authFetch(`/api/v1/comments/${comment.id}`, { method: 'DELETE' })
    message.success('已删除')
    await load(true)
  }
  catch {
    message.error('删除失败')
  }
  finally {
    busyIds.value.delete(comment.id)
  }
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
          loading: busyIds.value.has(row.id),
          disabled: busyIds.value.has(row.id) || loading.value,
        }, { default: () => '通过' }))
      }
      buttons.push(h(NPopconfirm, {
        onPositiveClick: () => removeComment(row),
        disabled: busyIds.value.has(row.id) || loading.value,
      }, {
        trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error', disabled: busyIds.value.has(row.id) || loading.value }, { default: () => '删除' }),
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

    <p class="text-sm text-slate-500 dark:text-slate-400">待审核列表集中展示需要人工处理的评论；已通过评论公开展示。</p>

    <NDataTable
      :columns="columns"
      :data="comments"
      :loading="loading"
      :bordered="false"
      size="small"
      :row-key="(row: Comment) => row.id"
    />
    <div class="flex items-center justify-end gap-3 text-sm">
      <NButton size="small" :disabled="loading || !!busyIds.size" @click="load(true)">刷新</NButton>
      <span class="text-slate-500 dark:text-slate-400">第 {{ pageIndex + 1 }} 页 · 本页 {{ comments.length }} 条</span>
      <NButton size="small" :disabled="loading || !!busyIds.size || pageIndex === 0" @click="previousPage">上一页</NButton>
      <NButton size="small" :disabled="loading || !!busyIds.size || !nextPageToken" @click="nextPage">下一页</NButton>
    </div>
  </div>
</template>
