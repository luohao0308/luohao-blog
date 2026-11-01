<script setup lang="ts">
import type { Article } from '~/composables/useArticles'
import { articleVersion } from '~/utils/articleVersion'
import { revisionDiff } from '~/utils/revisionDiff'
interface Revision { article_slug: string, version: string, action: string, actor_id?: string, created_at?: string, snapshot?: Article }
const props = defineProps<{ article: Article, baseVersion: string, canRestore: boolean, busy: boolean }>()
const emit = defineEmits<{ restore: [version: string] }>()
const { authFetch } = useAuth()
const rows = ref<Revision[]>([])
const selected = ref<Revision | null>(null)
const loading = ref(false)
const notice = ref('')
const nextToken = ref('')
const confirmation = ref(false)
let generation = 0, detailGeneration = 0
const seenTokens = new Set<string>()
const diff = computed(() => revisionDiff(selected.value?.snapshot?.content_md || '', props.article.content_md || ''))
const metadataLabels: Record<string, string> = { title: '标题', summary: '摘要', tags: '标签', category: '分类', cover_url: '封面', featured: '精选' }
const metadata = computed(() => ['title', 'summary', 'tags', 'category', 'cover_url', 'featured'].map(key => ({ key, before: JSON.stringify(selected.value?.snapshot?.[key as keyof Article] ?? ''), after: JSON.stringify(props.article[key as keyof Article] ?? '') })))
const restorable = computed(() => props.canRestore && !!selected.value?.snapshot && articleVersion(selected.value.version) !== props.baseVersion && !props.busy && !loading.value)
async function load(more = false) {
  const version = ++generation
  loading.value = true; notice.value = ''
  if (!more) { rows.value = []; nextToken.value = ''; seenTokens.clear(); selected.value = null; confirmation.value = false; detailGeneration++ }
  try {
    const result = await authFetch<{ revisions: Revision[], next_page_token?: string }>(`/api/v1/articles/${encodeURIComponent(props.article.slug)}/revisions`, { query: { page_size: 20, page_token: more ? nextToken.value : undefined } })
    if (version !== generation) return
    if (!Array.isArray(result.revisions) || result.revisions.some(row => row.article_slug !== props.article.slug || !articleVersion(row.version))) throw new Error('invalid revision list')
    const token = result.next_page_token || ''
    if (token && (seenTokens.has(token) || result.revisions.length === 0)) throw new Error('invalid revision pagination')
    if (token) seenTokens.add(token)
    rows.value = [...new Map((more ? [...rows.value, ...result.revisions] : result.revisions).map(row => [row.version, row])).values()]
    nextToken.value = token
  } catch { if (version === generation) notice.value = '历史版本暂时无法读取，请稍后重试。' }
  finally { if (version === generation) loading.value = false }
}
async function select(row: Revision) {
  const version = ++detailGeneration
  selected.value = null; confirmation.value = false; notice.value = ''
  try {
    const revision = await authFetch<Revision>(`/api/v1/articles/${encodeURIComponent(props.article.slug)}/revisions/${encodeURIComponent(row.version)}`)
    if (version !== detailGeneration) return
    if (revision.article_slug !== props.article.slug || revision.version !== row.version || !revision.snapshot) throw new Error('invalid revision detail')
    selected.value = revision
  } catch { if (version === detailGeneration) notice.value = '版本详情无法读取，请重新选择。' }
}
function download(text: string, label: string) {
  const url = URL.createObjectURL(new Blob([text], { type: 'text/markdown;charset=utf-8' }))
  const link = document.createElement('a'); link.href = url; link.download = `${label}.md`; link.click()
  setTimeout(() => URL.revokeObjectURL(url), 0)
}
onMounted(() => load())
watch(() => [props.article.slug, props.article.version], () => { void load() })
onBeforeUnmount(() => { generation++; detailGeneration++ })
defineExpose({ refresh: () => load() })
</script>

<template>
  <section aria-label="修订历史" class="ui-panel min-w-0 space-y-4 p-4">
    <div class="flex flex-wrap items-center justify-between gap-3"><h2 class="font-medium">修订历史</h2><button type="button" class="ui-link min-h-11" :disabled="loading || busy" @click="load()">刷新历史</button></div>
    <p class="text-xs text-[var(--text-muted)]">历史从功能接入后开始。接入基线沿用原文章更新时间，不代表一次作者保存操作。</p>
    <p v-if="notice" role="alert">{{ notice }}</p>
    <p v-if="loading" role="status">正在读取历史…</p>
    <p v-else-if="!rows.length && !notice">暂无历史记录，首次修改后会保留接入基线。</p>
    <div class="flex flex-wrap gap-3"><button v-for="row in rows" :key="row.version" type="button" class="ui-link min-h-11" :disabled="busy" @click="select(row)">查看版本 {{ row.version }} · {{ row.action === 'baseline' ? '接入基线' : row.action === 'restore' ? '恢复' : '保存' }}{{ row.version === baseVersion ? '（编辑基准）' : '' }}{{ row.created_at ? ` · ${row.created_at}` : '' }}</button></div>
    <button v-if="nextToken" type="button" class="ui-link min-h-11" :disabled="loading || busy" @click="load(true)">更多版本</button>
    <div v-if="selected?.snapshot" class="min-w-0 space-y-4">
      <h3 class="font-medium">版本 {{ selected.version }} 与当前编辑基准内容对照</h3>
      <dl class="space-y-2 text-sm"><div v-for="item in metadata.filter(item => item.before !== item.after)" :key="item.key" class="break-words"><dt>{{ metadataLabels[item.key] }}</dt><dd>历史：{{ item.before }}</dd><dd>当前：{{ item.after }}</dd></div></dl>
      <p v-if="diff.fallback" role="status" class="text-sm">内容超出差异计算上限，已切换并排文本；预览仅显示前 65536 个字符，可下载全文。</p>
      <div v-if="diff.fallback" class="grid min-w-0 gap-3 md:grid-cols-2"><pre class="max-h-96 min-w-0 overflow-auto whitespace-pre-wrap break-words text-xs">{{ selected.snapshot.content_md.slice(0, 65536) }}</pre><pre class="max-h-96 min-w-0 overflow-auto whitespace-pre-wrap break-words text-xs">{{ article.content_md.slice(0, 65536) }}</pre></div>
      <pre v-else data-testid="revision-diff" class="max-h-96 overflow-auto whitespace-pre-wrap break-words text-xs"><span v-for="(line, index) in diff.lines" :key="index" class="block" :class="line.kind === 'add' ? 'bg-emerald-50 text-emerald-900 dark:bg-emerald-950 dark:text-emerald-100' : line.kind === 'remove' ? 'bg-rose-50 text-rose-900 dark:bg-rose-950 dark:text-rose-100' : ''">{{ line.kind === 'add' ? '+' : line.kind === 'remove' ? '-' : ' ' }} {{ line.text }}</span></pre>
      <div class="flex flex-wrap gap-4"><button type="button" class="ui-link min-h-11 text-sm" @click="download(selected.snapshot.content_md, `revision-${selected.version}`)">下载历史全文</button><button type="button" class="ui-link min-h-11 text-sm" @click="download(article.content_md, 'current')">下载当前全文</button></div>
      <p v-if="!canRestore" class="text-sm text-amber-700 dark:text-amber-300">有未保存修改、待恢复草稿或上传操作，请先保存或在新标签对照后整理。</p>
      <label class="flex items-start gap-2 text-sm"><input v-model="confirmation" type="checkbox" :disabled="!restorable" class="mt-1">已核对历史内容；恢复将保存为新版本，保持当前发布状态，并更新已发布文章的内容。</label>
      <button type="button" class="ui-button min-h-11 px-3 text-sm" :disabled="!restorable || !confirmation" @click="emit('restore', selected.version)">确认恢复为新版本</button>
    </div>
  </section>
</template>
