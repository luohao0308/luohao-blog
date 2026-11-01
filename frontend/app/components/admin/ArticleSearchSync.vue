<script setup lang="ts">
import { NButton } from 'naive-ui'

interface SearchSyncStatus {
  article_slug: string
  state: number
  can_retry: boolean
  message: string
  checked_at?: string
  semantic_ready?: boolean
}
const props = defineProps<{ slug: string }>()
const { authFetch } = useAuth()
const status = ref<SearchSyncStatus | null>(null)
const busy = ref(false)
const notice = ref('')
let generation = 0
const labels: Record<number, string> = {
  0: '状态未核实', 1: '索引已同步', 2: '索引缺失', 3: '索引待更新',
  4: '无需索引', 5: '暂时无法核实', 6: '部分同步',
}
async function check(retry = false) {
  const version = ++generation
  busy.value = true
  notice.value = ''
  try {
    const endpoint = `/api/v1/articles/${encodeURIComponent(props.slug)}/search-sync${retry ? '/retry' : ''}`
    const result = await authFetch<SearchSyncStatus>(endpoint, retry ? { method: 'POST', body: {} } : {})
    if (version !== generation) return
    if (result.article_slug !== props.slug || typeof result.state !== 'number'
      || !Object.hasOwn(labels, result.state) || typeof result.can_retry !== 'boolean'
      || typeof result.message !== 'string') throw new Error('invalid sync status')
    status.value = result
  }
  catch (error: unknown) {
    if (version !== generation) return
    status.value = null
    const code = (error as { response?: { status?: number } })?.response?.status
    notice.value = code === 403
      ? '无权查询或重试搜索同步。'
      : retry ? '本次重试未确认成功，请刷新状态后再试。正文保存不受影响。'
        : '搜索同步状态暂时无法获取，可稍后刷新；正文保存不受影响。'
  }
  finally {
    if (version === generation) busy.value = false
  }
}
function refresh() { return check() }
function retry() {
  if (!busy.value && status.value?.can_retry === true) return check(true)
}
onMounted(refresh)
watch(() => props.slug, () => { status.value = null; void refresh() })
onBeforeUnmount(() => { generation++ })
defineExpose({ refresh })
</script>

<template>
  <section aria-label="搜索同步" class="space-y-3 rounded-xl border border-slate-200 p-4 dark:border-slate-700">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="font-medium">搜索同步</h2>
      <div class="flex flex-wrap gap-2">
        <NButton size="small" :disabled="busy" @click="refresh">刷新状态</NButton>
        <NButton v-if="status?.can_retry" size="small" type="primary" :loading="busy" :disabled="busy" @click="retry">重试同步</NButton>
      </div>
    </div>
    <p v-if="busy" role="status" class="text-sm text-slate-500">正在核验或同步索引…</p>
    <p v-else-if="notice" role="alert" class="text-sm text-amber-700 dark:text-amber-300">{{ notice }}</p>
    <div v-else-if="status" role="status" class="space-y-1 text-sm">
      <p :class="status.state === 1 ? 'text-emerald-700 dark:text-emerald-300' : status.state === 4 ? 'text-slate-500' : 'text-amber-700 dark:text-amber-300'">{{ labels[status.state] }}</p>
      <p class="break-words text-slate-600 dark:text-slate-400">{{ status.message }}</p>
    </div>
    <p class="text-xs text-slate-500">按服务器已保存的正文核验当前索引；未保存的编辑不会参与同步。</p>
  </section>
</template>
