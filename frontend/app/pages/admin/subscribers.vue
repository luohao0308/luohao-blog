<script setup lang="ts">
// Subscription management: the registered email ledger. Delete removes the
// row; actual sending is not wired yet (registration-only feature).
import { NButton, NDataTable, NPopconfirm, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'

import { formatDate } from '~/composables/useArticles'

definePageMeta({ layout: 'admin', middleware: 'admin-auth' })

useHead({ title: '订阅管理' })

const { authFetch } = useAuth()
const message = useMessage()

interface Subscriber {
  id: string
  email: string
  created_at?: import('~/composables/useArticles').ArticleTimestamp
}

const subscribers = ref<Subscriber[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const set = await authFetch<{ subscribers: Subscriber[] }>('/api/v1/subscribers/list', {
      query: { page_size: 100 },
    })
    subscribers.value = set.subscribers ?? []
  }
  catch {
    message.error('订阅列表加载失败')
  }
  finally {
    loading.value = false
  }
}

onMounted(load)

async function remove(subscriber: Subscriber) {
  try {
    await authFetch(`/api/v1/subscribers/${encodeURIComponent(subscriber.email)}`, { method: 'DELETE' })
    message.success('已删除')
  }
  catch {
    message.error('删除失败')
  }
  load()
}

const columns: DataTableColumns<Subscriber> = [
  {
    title: '邮箱',
    key: 'email',
    render: row => h('code', { class: 'text-xs text-slate-700 dark:text-slate-200' }, { default: () => row.email }),
  },
  {
    title: '订阅时间',
    key: 'created_at',
    width: 140,
    render: row => formatDate(row.created_at),
  },
  {
    title: '操作',
    key: 'actions',
    width: 120,
    render: row => h(NPopconfirm, {
      onPositiveClick: () => remove(row),
    }, {
      trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, { default: () => '删除' }),
      default: () => `移除 ${row.email} 的订阅？`,
    }),
  },
]
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-xl font-semibold">
        订阅管理
      </h1>
      <NButton size="small" quaternary @click="load()">
        刷新
      </NButton>
    </div>
    <p class="text-sm text-slate-500 dark:text-slate-400">
      已登记的订阅邮箱。当前仅做登记，邮件发送尚未接入。
    </p>

    <NDataTable
      :columns="columns"
      :data="subscribers"
      :loading="loading"
      :bordered="false"
      size="small"
      :row-key="(row: Subscriber) => row.email"
    />
  </div>
</template>
