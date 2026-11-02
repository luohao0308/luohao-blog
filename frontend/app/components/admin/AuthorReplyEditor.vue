<script setup lang="ts">
import { NPopconfirm } from 'naive-ui'
import type { Comment } from '~/composables/useComments'
const props = defineProps<{ root: Comment }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const { user, authFetch } = useAuth()
const editorAccount = user.value?.id
const editorRoot = props.root.id
const text = ref('')
const current = ref<Comment | null>(null)
const ready = ref(false)
const busy = ref(false)
const notice = ref('')
let generation = 0
let resetting = false
const drafts = useState<Record<string, string>>('author-reply-drafts', () => ({}))
const key = computed(() => `${user.value?.id ?? ''}:${props.root.id}`)
const editable = computed(() => !current.value || current.value.user_id === user.value?.id)
watch(text, value => { if (!resetting && user.value?.id === editorAccount && props.root.id === editorRoot) drafts.value[key.value] = value }, { flush: 'sync' })
watch(() => user.value?.id, () => { resetting = true; generation++; text.value = ''; current.value = null; ready.value = false; busy.value = false; notice.value = ''; resetting = false; emit('close') })
watch(() => props.root.id, () => { resetting = true; generation++; text.value = ''; current.value = null; ready.value = false; busy.value = false; resetting = false; emit('close') })
async function load() {
  if (busy.value) return
  const owner = user.value?.id
  const version = ++generation
  const savedDraft = drafts.value[key.value]
  busy.value = true
  ready.value = false
  try {
    let reply: Comment | null = null
    try { reply = await authFetch<Comment>(`/api/v1/comments/${props.root.id}/author-reply`, { expectedUserId: owner }) }
    catch (error: unknown) { if ((error as { response?: { status?: number } }).response?.status !== 404) throw error }
    if (version !== generation || owner !== user.value?.id) return
    current.value = reply
    text.value = savedDraft ?? reply?.content ?? ''
    ready.value = true
    notice.value = savedDraft !== undefined ? '已加载最新版本，保留你的草稿，请比较后保存。' : ''
  }
  catch { if (version === generation) notice.value = '回复加载失败，请重试。' }
  finally { if (version === generation) busy.value = false }
}
async function save() {
  if (!ready.value || busy.value || !editable.value || !text.value.trim()) return
  if (Array.from(text.value.trim()).length > 1000) { notice.value = '回复最多 1000 字。'; return }
  const owner = user.value?.id, version = generation, draftKey = key.value, body = text.value
  busy.value = true
  try {
    const result = await authFetch<Comment>(`/api/v1/comments/${props.root.id}/author-reply`, { expectedUserId: owner, method: 'PUT', body: { content: body.trim(), expected_reply_version: String(current.value?.reply_version ?? '0') } })
    if (version !== generation || owner !== user.value?.id) return
    current.value = result
    if (text.value === body) drafts.value = Object.fromEntries(Object.entries(drafts.value).filter(([key]) => key !== draftKey))
    notice.value = result.status === 2 ? '作者回复已公开。' : '回复已保存，当前尚未公开。'
    emit('saved')
  }
  catch (error: unknown) {
    if (version !== generation || owner !== user.value?.id) return
    const status = (error as { response?: { status?: number } }).response?.status
    if (status === 409) ready.value = false
    notice.value = status === 409 ? '回复已被更新。草稿已保留，请加载最新版本比较后再保存。' : status === 400 ? '回复未通过自动检测，请修改后重试。' : '保存失败，草稿已保留。'
  }
  finally { if (version === generation) busy.value = false }
}
async function remove() {
  if (!current.value || busy.value) return
  const owner = user.value?.id, version = generation, id = current.value.id
  busy.value = true
  try {
    await authFetch(`/api/v1/comments/${id}`, { expectedUserId: owner, method: 'DELETE' })
    if (version !== generation || owner !== user.value?.id) return
    current.value = null
    ready.value = true
    notice.value = '作者回复已删除，原评论保留。'
    emit('saved')
  }
  catch { if (version === generation) notice.value = '删除失败，请加载最新版本确认。' }
  finally { if (version === generation) busy.value = false }
}
onMounted(load)
onBeforeUnmount(() => { generation++ })
</script>
<template>
  <section class="space-y-3 rounded-xl border border-slate-300 p-4 dark:border-slate-700" aria-label="编辑作者回复">
    <h2 class="font-medium">作者回复</h2>
    <p class="whitespace-pre-wrap break-words text-sm text-slate-500">{{ root.content }}</p>
    <p v-if="current && !editable" class="text-sm">该回复由其他管理员撰写，仅原作者可编辑。</p>
    <p v-if="current" class="whitespace-pre-wrap break-words text-sm">当前回复：{{ current.content }}</p>
    <textarea v-model="text" aria-label="回复内容" rows="4" :disabled="busy || !editable" class="w-full rounded border border-slate-300 bg-transparent p-2" />
    <p v-if="notice" role="status" class="text-sm">{{ notice }}</p>
    <div class="flex gap-4 text-sm">
      <button :disabled="busy || !ready || !editable || !text.trim()" class="disabled:opacity-40" @click="save">{{ busy ? '处理中…' : '保存回复' }}</button>
      <button :disabled="busy" @click="load">加载最新版本</button>
      <NPopconfirm v-if="current" :disabled="busy" @positive-click="remove"><template #trigger><button :disabled="busy">删除作者回复</button></template>确定只删除作者回复吗？原评论将保留。</NPopconfirm>
      <button :disabled="busy" @click="emit('close')">关闭</button>
    </div>
  </section>
</template>
