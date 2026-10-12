<script setup lang="ts">
import { NButton, NDataTable, NInput, NTag, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { formatDate } from '~/composables/useArticles'

definePageMeta({ layout: 'admin', middleware: 'admin-auth' })
useHead({ title: '公众号同步' })
const { authFetch } = useAuth()
const message = useMessage()
interface Settings { enabled: boolean, configured: boolean, mode: string, account_name: string }
interface SyncTask {
  id: string
  slug: string
  title: string
  state: string
  mode: string
  last_error: string
  article_url: string
  can_retry: boolean
  next_attempt_at?: import('~/composables/useArticles').ArticleTimestamp
  updated_at?: import('~/composables/useArticles').ArticleTimestamp
}
const settings = ref<Settings | null>(null)
const tasks = ref<SyncTask[]>([])
const nextPageToken = ref('')
const loading = ref(false)
const error = ref('')
const slug = ref('')
const enqueueing = ref(false)
const retrying = ref<string | null>(null)
const modes: Record<string, string> = { draft: '只同步草稿', publish: '公开发表', mass: '推送关注者' }
const states: Record<string, string> = {
  queued: '等待同步', drafting: '生成草稿中', ready: '草稿已就绪', submitting: '提交中',
  polling: '等待微信结果', checking: '查询结果中', drafted: '已同步草稿', succeeded: '同步成功',
  failed: '同步失败', unknown: '结果未知，需核对', cancelled: '文章已变更，取消同步',
}
const errors: Record<string, string> = {
  title_too_long: '标题超过公众号的 32 字限制，请调整后同步新版本。',
  title_required: '文章标题为空。',
  content_too_large: '正文超过公众号大小限制。',
  cover_required: '尚未配置公众号封面素材。',
  image_host_not_allowed: '正文图片域名未加入允许列表。',
  image_host_not_public: '正文图片地址不是公开地址。',
  image_type_unsupported: '正文图片需为 JPG 或 PNG。',
  image_too_large: '正文图片需小于 1MB。',
  recovery_ambiguous: '上次同步中断，需到公众号后台核对。',
  target_account_changed: '公众号已变更，原任务需到原账号核对。',
  '45009': '微信调用次数已达上限，请额度恢复后重试。',
  '48001': '公众号缺少该接口权限。',
  '40014': '调用凭据失效，已刷新并等待重查。',
  '42001': '调用凭据过期，已刷新并等待重查。',
  transport_error: '网络暂时不可用，等待退避后重查。',
  unknown: '微信请求结果未知，需到公众号后台核对。',
}
const active = computed(() => settings.value?.enabled && settings.value.configured)
async function load(more = false) {
  if (loading.value)
    return
  loading.value = true
  error.value = ''
  try {
    const [config, result] = await Promise.all([
      authFetch<Settings>('/api/v1/admin/wechat-sync/settings'),
      authFetch<{ tasks?: SyncTask[], next_page_token?: string }>('/api/v1/admin/wechat-sync/tasks', {
        query: { page_size: 20, page_token: more ? nextPageToken.value : undefined },
      }),
    ])
    settings.value = config
    tasks.value = more ? [...tasks.value, ...(result.tasks ?? [])] : (result.tasks ?? [])
    nextPageToken.value = result.next_page_token ?? ''
  }
  catch {
    error.value = '公众号同步状态加载失败，请重试。'
  }
  finally {
    loading.value = false
  }
}
onMounted(() => load())
async function enqueue() {
  if (!slug.value.trim() || enqueueing.value || !active.value)
    return
  enqueueing.value = true
  try {
    await authFetch(`/api/v1/admin/wechat-sync/articles/${encodeURIComponent(slug.value.trim())}`, { method: 'POST', body: {} })
    message.success('已加入同步任务，同一文章版本不会重复创建任务。')
    slug.value = ''
    await load()
  }
  catch {
    message.error('同步未加入队列：请确认配置已启用且文章已发布。')
  }
  finally {
    enqueueing.value = false
  }
}
async function retry(task: SyncTask) {
  if (retrying.value)
    return
  retrying.value = task.id
  try {
    await authFetch(`/api/v1/admin/wechat-sync/tasks/${task.id}/retry`, { method: 'POST', body: {} })
    message.success('已重新加入队列')
    await load()
  }
  catch {
    message.error('无法重试：配置或文章已变化，或任务不能安全重试。')
  }
  finally {
    retrying.value = null
  }
}
const columns: DataTableColumns<SyncTask> = [
  { title: '文章', key: 'title', minWidth: 180, render: row => h(resolveComponent('NuxtLink'), { to: `/admin/posts/${row.slug}/edit` }, () => row.title) },
  { title: '方式', key: 'mode', width: 130, render: row => modes[row.mode] ?? row.mode },
  { title: '状态', key: 'state', width: 190, render: row => h(NTag, { size: 'small', type: row.state === 'succeeded' || row.state === 'drafted' ? 'success' : row.state === 'unknown' || row.state === 'failed' ? 'warning' : 'default' }, () => states[row.state] ?? row.state) },
  { title: '最近更新', key: 'updated_at', width: 120, render: row => formatDate(row.updated_at) },
  { title: '下次查询', key: 'next_attempt_at', width: 160, render: row => typeof row.next_attempt_at === 'string' ? new Date(row.next_attempt_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) : '—' },
  { title: '错误码', key: 'last_error', minWidth: 180, render: row => errors[row.last_error] ?? (row.last_error || '—') },
  { title: '操作', key: 'actions', width: 100, render: row => row.can_retry ? h(NButton, { size: 'small', disabled: !active.value || retrying.value !== null, loading: retrying.value === row.id, onClick: () => retry(row) }, () => '重试') : row.article_url && /^https:\/\/mp\.weixin\.qq\.com\//.test(row.article_url) ? h('a', { href: row.article_url, target: '_blank', rel: 'noopener', class: 'text-blue-600' }, '查看文章') : '—' },
]
</script>

<template>
  <div class="space-y-6">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold">公众号同步</h1>
        <p class="mt-1 text-sm text-slate-500">{{ settings?.account_name || '公众号' }} · {{ settings ? modes[settings.mode] : '加载中…' }}</p>
      </div>
      <NButton :loading="loading" @click="load()">刷新状态</NButton>
    </header>
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    <section class="space-y-2 rounded-xl border border-slate-200 p-5 dark:border-slate-700">
      <h2 class="font-medium">{{ active ? '自动同步已启用' : '自动同步尚未启用' }}</h2>
      <p class="text-sm text-slate-500 dark:text-slate-400">
        新发布的文章自动排队，微信接口失败不会影响博客阅读。修改已发布文章不会再次通知关注者，可在下方手动同步新的版本。
      </p>
      <p v-if="settings && !active" class="text-sm text-slate-500 dark:text-slate-400">
        需要在服务端配置公众号 AppID、AppSecret、博客公开地址和永久封面素材 ID，然后启用同步。凭据不会在此页面显示。
      </p>
      <p class="text-sm text-slate-500 dark:text-slate-400">
        “公开发表”与“推送关注者”是不同方式。等待微信结果不代表已经发送成功；查询失败会延迟重试，永久错误会停止查询。结果未知的任务需要到公众号后台核对，不能直接重试。
      </p>
    </section>
    <form class="flex flex-col gap-3 sm:flex-row" @submit.prevent="enqueue">
      <label for="wechat-article-slug" class="sr-only">已发布文章的 slug</label>
      <NInput v-model:value="slug" :input-props="{ id: 'wechat-article-slug' }" placeholder="已发布文章的 slug，例如 ai-delivery-guard" :disabled="!active" />
      <NButton type="primary" attr-type="submit" :disabled="!active || !slug.trim()" :loading="enqueueing">同步文章</NButton>
    </form>
    <NDataTable :columns="columns" :data="tasks" :loading="loading" :bordered="false" :scroll-x="1000" :row-key="(row: SyncTask) => row.id" />
    <NButton v-if="nextPageToken" :loading="loading" @click="load(true)">加载更多</NButton>
  </div>
</template>
