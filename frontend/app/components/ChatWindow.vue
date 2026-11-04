<script setup lang="ts">
import type { ChatRequestStatus } from '~/composables/useChatArticle'
interface CitedArticle { slug: string, title: string, summary?: string, section_title?: string, anchor?: string, excerpt?: string }
interface Reply { answer?: string, citations?: CitedArticle[], outcome?: number | string, article_slug?: string }
interface Question {
  id: number
  query: string
  state: 'pending' | 'returned' | 'failed'
  reply?: Reply
  error?: string
  copyNotice?: string
}
const props = defineProps<{ articleSlug?: string, embedded?: boolean, variant?: 'bubble' | 'wechat' }>()
const emit = defineEmits<{ status: [value: ChatRequestStatus] }>()
let generation = 0
const questions = ref<Question[]>([])
const input = ref('')
const history = ref<HTMLElement | null>(null)
const suggestions = computed(() => props.articleSlug
  ? ['这篇文章讲了什么？', '能用一个例子解释吗？', '有哪些要注意的地方？']
  : ['这个博客写些什么？', '有哪些项目可以看看？', '推荐几篇后端文章'])
const composer = ref<HTMLTextAreaElement | null>(null)
const busy = ref(false)
const composing = ref(false)
const inputSize = computed(() => Array.from(input.value.trim()).length)
const canAsk = computed(() => !busy.value && inputSize.value > 0 && inputSize.value <= 500)
const copyTimers = new Map<number, ReturnType<typeof setTimeout>>()
let sequence = 0
let controller: AbortController | undefined
let disposed = false

watch(input, async () => {
  await nextTick()
  if (props.embedded || !composer.value) return
  composer.value.style.height = 'auto'
  composer.value.style.height = `${Math.min(240, Math.max(64, composer.value.scrollHeight))}px`
})

// Old servers have no outcome: keep their result readable without guessing
// generation success from HTTP status or the answer text.
const outcomeNames = ['CHAT_OUTCOME_UNSPECIFIED', 'CHAT_OUTCOME_ANSWERED', 'CHAT_OUTCOME_NO_MATCH', 'CHAT_OUTCOME_RETRIEVAL_UNAVAILABLE', 'CHAT_OUTCOME_GENERATION_UNAVAILABLE']
function outcome(reply?: Reply): number {
  if (typeof reply?.outcome === 'number') return reply.outcome
  return outcomeNames.indexOf(reply?.outcome ?? '')
}
function resultLabel(reply?: Reply): string {
  switch (outcome(reply)) {
    case 1: return '回答已生成'
    case 2: return '没有找到相关内容，可换个说法再试。'
    case 3: return '检索服务暂时不可用，请稍后重试。'
    case 4: return '回答生成暂时不可用，可以先阅读参考文章。'
    default: return '返回结果'
  }
}
function retryable(question: Question): boolean {
  return question.state === 'failed' || [3, 4].includes(outcome(question.reply))
}
async function send(question: Question) {
  if (busy.value) return
  busy.value = true
  emit('status', 'working')
  question.state = 'pending'
  question.error = ''
  const version = generation
  const requestedArticleSlug = props.articleSlug
  controller = new AbortController()
  try {
    const reply = await $fetch<Reply>('/api/v1/chat', {
      method: 'POST', body: { query: question.query, ...(requestedArticleSlug ? { article_slug: requestedArticleSlug } : {}) }, signal: controller.signal,
      // A retry of a paid request is always an explicit reader action.
      retry: 0,
      timeout: 130_000,
    })
    if (disposed || version !== generation) return
    if (requestedArticleSlug && reply.article_slug !== requestedArticleSlug) {
      question.error = '当前服务暂不支持问本文，请稍后重试。'
      question.state = 'failed'
      emit('status', 'failure')
      return
    }
    if (!requestedArticleSlug && reply.article_slug) {
      question.error = '回答范围不匹配，请重试。'
      question.state = 'failed'
      emit('status', 'failure')
      return
    }
    if ((outcome(reply) === 1 && !reply.answer?.trim()) || (!reply.answer?.trim() && !reply.citations?.length && ![2, 3, 4].includes(outcome(reply))))
      throw new Error('Empty chat result')
    question.reply = reply
    question.state = 'returned'
    emit('status', outcome(reply) === 1 ? 'done' : [3, 4].includes(outcome(reply)) ? 'failure' : 'idle')
  }
  catch (err: unknown) {
    if (disposed || version !== generation) return
    const status = (err as { response?: { status?: number } })?.response?.status
    question.error = status === 429 ? '提问太频繁了，请稍后再试。'
      : status === 400 ? '问题不符合要求，请编辑后重试（最多 500 字）。'
        : status === 404 ? '这篇文章已不可用，请刷新页面后重试。'
        : status === 503 ? '问答服务暂时不可用，请稍后重试。'
          : '回答请求失败，问题已保留，可以重试。'
    question.state = 'failed'
    emit('status', 'failure')
  }
  finally {
    if (!disposed && version === generation) {
      busy.value = false
      controller = undefined
    }
  }
}
async function ask() {
  if (!canAsk.value || composing.value) return
  const question: Question = reactive({ id: ++sequence, query: input.value.trim(), state: 'pending' })
  questions.value.push(question)
  input.value = ''
  await send(question)
}
function askSuggestion(query: string) {
  if (busy.value || composing.value || input.value.trim()) return
  input.value = query
  void ask()
}
watch(() => [questions.value.length, busy.value], async () => {
  await nextTick()
  if (history.value) history.value.scrollTop = history.value.scrollHeight
})
function citationPath(citation: CitedArticle) {
  const path = `/posts/${encodeURIComponent(citation.slug)}`
  const found = citation.anchor && (!props.articleSlug || citation.slug !== props.articleSlug
    || (import.meta.client && [...document.querySelectorAll('.markdown-body h2, .markdown-body h3')].some(h => h.id === citation.anchor)))
  return found ? `${path}#${encodeURIComponent(citation.anchor!)}` : path
}
watch(() => props.articleSlug, () => {
  generation++
  controller?.abort()
  busy.value = false
  clear()
  input.value = ''
  emit('status', 'idle')
})
function onKeydown(event: KeyboardEvent) {
  if (event.isComposing || composing.value || event.keyCode === 229) return
  if (event.key === 'Enter' && !event.shiftKey && !event.ctrlKey && !event.metaKey && !event.altKey) {
    event.preventDefault()
    ask()
  }
}
function edit(question: Question) {
  if (input.value.trim()) return // Never overwrite a prepared next question.
  input.value = question.query
  composer.value?.focus()
}
async function copy(question: Question) {
  clearTimeout(copyTimers.get(question.id))
  try {
    await navigator.clipboard.writeText(question.reply?.answer ?? '')
    if (disposed) return
    question.copyNotice = '回答已复制'
  }
  catch {
    if (disposed) return
    question.copyNotice = '复制失败，请选择文字手动复制。'
  }
  copyTimers.set(question.id, setTimeout(() => { question.copyNotice = '' }, 3000))
}
function clear() {
  if (busy.value) return
  questions.value = []
  for (const timer of copyTimers.values()) clearTimeout(timer)
  copyTimers.clear()
  emit('status', 'idle')
}
onBeforeUnmount(() => {
  disposed = true
  generation++
  controller?.abort()
  for (const timer of copyTimers.values()) clearTimeout(timer)
  emit('status', 'idle')
})
</script>

<template>
  <section class="space-y-5" :data-variant="variant" :class="{ 'assistant-chat': embedded, 'border-t border-[var(--border-default)] py-6': !embedded }" :aria-label="embedded ? '对话记录' : 'AI 问答'">
    <header v-if="!embedded" class="flex flex-wrap items-start justify-between gap-3">
      <div v-if="!embedded">
        <h2 class="text-lg font-medium">{{ articleSlug ? '问本文' : '问一问' }}</h2>
        <p class="mt-1 text-sm leading-6 text-[var(--text-muted)]">{{ articleSlug ? '可以聊聊这篇文章，回答会附上参考段落。' : '可以帮你找文章，也可以问问文章里的内容。' }}</p>
      </div>
      <button v-if="questions.length" type="button" :disabled="busy" class="ui-link min-h-11 px-2 text-sm disabled:opacity-50" @click="clear">清空记录</button>
    </header>
    <div v-if="embedded && !questions.length" class="assistant-intro">
      <p>{{ variant === 'bubble' ? '想聊点什么？' : '你好！想了解这个博客的什么？' }}</p>
      <div class="assistant-suggestions">
        <button v-for="suggestion in suggestions.slice(0, variant === 'bubble' ? 2 : 3)" :key="suggestion" type="button" :disabled="!!input.trim() || composing" @click="askSuggestion(suggestion)">{{ suggestion }}</button>
      </div>
    </div>
    <div v-if="questions.length" ref="history" class="space-y-4 assistant-history" role="log" aria-label="聊天记录">
      <div v-for="question in questions" :key="question.id" class="min-w-0 space-y-3">
        <p data-testid="chat-question" class="chat-query">{{ question.query }}</p>
        <p v-if="question.state === 'pending'" role="status" class="text-sm text-[var(--text-muted)]">正在回复…</p>
        <div v-else-if="question.state === 'returned'" class="space-y-3">
          <p v-if="!embedded || outcome(question.reply) !== 1" role="status" class="text-xs text-[var(--text-muted)]">{{ resultLabel(question.reply) }}</p>
          <p v-if="question.reply?.answer" data-testid="chat-answer" class="assistant-reply whitespace-pre-wrap break-words text-sm leading-7">{{ question.reply.answer }}</p>
          <div v-if="question.reply?.citations?.length" class="space-y-2">
            <h3 class="text-xs font-medium text-[var(--text-muted)]">{{ articleSlug ? '参考段落' : '参考文章' }}</h3>
            <div class="citation-list grid min-w-0 gap-2" :class="{ 'sm:grid-cols-2': !embedded }">
              <NuxtLink v-for="citation in question.reply.citations" :key="`${citation.slug}:${citation.anchor || citation.section_title || citation.title}`" :to="citationPath(citation)" class="ui-panel min-w-0 space-y-1 p-3 hover:border-[var(--accent)]">
                <span class="ui-link block break-words text-sm font-medium">{{ citation.section_title || citation.title }}</span>
                <span v-if="citation.excerpt" class="block whitespace-pre-wrap break-words text-xs leading-6 text-[var(--text-muted)]">{{ citation.excerpt }}</span>
                <span v-else-if="citation.summary" class="line-clamp-2 block text-xs leading-6 text-[var(--text-muted)]">{{ citation.summary }}</span>
              </NuxtLink>
            </div>
          </div>
          <button v-if="!embedded && question.reply?.answer" type="button" class="ui-link min-h-11 text-xs" @click="copy(question)">复制回答</button>
          <p v-if="question.copyNotice" role="status" class="text-xs text-[var(--text-muted)]">{{ question.copyNotice }}</p>
        </div>
        <p v-else role="alert" class="text-sm text-[var(--status-error)]">{{ question.error }}</p>
        <div v-if="retryable(question)" class="flex flex-wrap gap-3 text-sm">
          <button type="button" :disabled="busy" class="ui-link min-h-11 disabled:opacity-50" @click="send(question)">重试问题</button>
          <button v-if="!embedded" type="button" :disabled="busy || !!input.trim()" :title="input.trim() ? '先发送或清空当前草稿，再编辑这个问题' : undefined" class="ui-link min-h-11 disabled:opacity-50" @click="edit(question)">编辑问题</button>
        </div>
      </div>
    </div>
    <form class="chat-composer" @submit.prevent="ask">
      <textarea id="blog-question" ref="composer" v-model="input" aria-label="输入消息" aria-describedby="blog-question-help" rows="1" placeholder="输入消息…" class="chat-input" @keydown="onKeydown" @compositionstart="composing = true" @compositionend="composing = false" />
      <button type="submit" :disabled="!canAsk || composing" class="chat-send" aria-label="发送" title="发送">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 19V5m-6 6 6-6 6 6" /></svg>
      </button>
    </form>
    <p id="blog-question-help" class="chat-disclosure" :class="{ 'is-over-limit': inputSize > 500 }">{{ inputSize > 500 ? '消息最多 500 字' : 'AI 根据公开文章回答 · 每条消息独立处理' }}</p>
  </section>
</template>

<style scoped>
.assistant-chat { display: flex; flex-direction: column; gap: .625rem; }
.assistant-chat > * { margin-block: 0; }
.assistant-intro { display: grid; gap: .625rem; font-size: .8125rem; }
.assistant-suggestions { display: flex; flex-wrap: wrap; gap: .375rem; }
.assistant-suggestions button { min-height: 32px; padding: .25rem .625rem; border: 1px solid var(--border-default); border-radius: .625rem; background: var(--surface-page); font-size: .75rem; text-align: left; }
.assistant-suggestions button:hover:not(:disabled) { background: var(--surface-hover); }
.assistant-suggestions button:disabled { opacity: .55; }
.assistant-chat .assistant-history { max-height: min(38dvh, 16rem); overflow-y: auto; overscroll-behavior: contain; padding: .25rem .25rem .25rem 0; }
.chat-query, .assistant-reply { width: fit-content; max-width: 92%; padding: .5rem .75rem; border-radius: .75rem; white-space: pre-wrap; overflow-wrap: anywhere; font-size: .8125rem; line-height: 1.65; }
.chat-query { margin-left: auto; background: var(--surface-accent); color: var(--text-primary); border-bottom-right-radius: .25rem; }
.assistant-reply { background: var(--surface-hover); border-bottom-left-radius: .25rem; }
.chat-composer { display: flex; align-items: center; gap: .5rem; padding: .1875rem .25rem .1875rem .625rem; border: 1px solid var(--border-default); border-radius: .625rem; background: var(--surface-page); }
.chat-composer:focus-within { border-color: var(--text-muted); }
.chat-input { display: block; width: 100%; min-width: 0; height: 2.5rem; padding: .5rem 0; resize: none; border: 0; outline: none; background: transparent; color: var(--text-primary); font-size: 16px; line-height: 1.5; }
.chat-input:focus-visible { outline: none; }
.chat-input::placeholder { color: var(--text-muted); opacity: 1; }
.chat-send { display: inline-flex; flex: none; align-items: center; justify-content: center; width: 44px; height: 44px; border-radius: .5rem; color: var(--on-accent); background: var(--accent); }
.chat-send:disabled { background: var(--surface-hover); color: var(--text-muted); }
.chat-disclosure { color: var(--text-muted); font-size: 11px; line-height: 1.6; }
.chat-disclosure.is-over-limit { color: var(--status-error); }
.assistant-chat[data-variant='bubble'] .assistant-suggestions { display: grid; }
.assistant-chat[data-variant='bubble'] .assistant-suggestions button { background: color-mix(in srgb, var(--surface-overlay) 50%, transparent); border-color: color-mix(in srgb, var(--text-muted) 22%, transparent); }
.assistant-chat[data-variant='bubble'] .chat-query { background: color-mix(in srgb, #e6b94f 20%, var(--surface-overlay)); }
.assistant-chat[data-variant='wechat'] { gap: 0; background: var(--surface-hover); }
.assistant-chat[data-variant='wechat'] .assistant-intro { margin: .75rem; padding: .625rem; border-radius: .75rem .75rem .75rem .25rem; background: var(--surface-overlay); }
.assistant-chat[data-variant='wechat'] .assistant-history { padding: .75rem; }
.assistant-chat[data-variant='wechat'] .assistant-reply { background: var(--surface-overlay); }
.assistant-chat[data-variant='wechat'] .chat-query { background: color-mix(in srgb, #83cf72 36%, var(--surface-overlay)); }
.assistant-chat[data-variant='wechat'] .chat-composer { margin: .375rem .625rem .25rem; padding: 2px 3px 2px .625rem; background: var(--surface-overlay); }
.assistant-chat[data-variant='wechat'] .chat-input { height: 34px; padding-block: 5px; }
.assistant-chat[data-variant='wechat'] .chat-send { width: 34px; height: 34px; }
.assistant-chat[data-variant='wechat'] .chat-send svg { width: 16px; height: 16px; }
.assistant-chat[data-variant='wechat'] .chat-disclosure { padding: .25rem .625rem .625rem; }
</style>
