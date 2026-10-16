<script setup lang="ts">
interface CitedArticle { slug: string, title: string, summary?: string }
interface Reply { answer?: string, citations?: CitedArticle[], outcome?: number | string }
interface Question {
  id: number
  query: string
  state: 'pending' | 'returned' | 'failed'
  reply?: Reply
  error?: string
  copyNotice?: string
}
const questions = ref<Question[]>([])
const input = ref('')
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
  if (!composer.value) return
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
  question.state = 'pending'
  question.error = ''
  controller = new AbortController()
  try {
    const reply = await $fetch<Reply>('/api/v1/chat', {
      method: 'POST', body: { query: question.query }, signal: controller.signal,
      // A retry of a paid request is always an explicit reader action.
      retry: 0,
      timeout: 130_000,
    })
    if (disposed) return
    if ((outcome(reply) === 1 && !reply.answer?.trim()) || (!reply.answer?.trim() && !reply.citations?.length && ![2, 3, 4].includes(outcome(reply))))
      throw new Error('Empty chat result')
    question.reply = reply
    question.state = 'returned'
  }
  catch (err: unknown) {
    if (disposed) return
    const status = (err as { response?: { status?: number } })?.response?.status
    question.error = status === 429 ? '提问太频繁了，请稍后再试。'
      : status === 400 ? '问题不符合要求，请编辑后重试（最多 500 字）。'
        : status === 503 ? '问答服务暂时不可用，请稍后重试。'
          : '回答请求失败，问题已保留，可以重试。'
    question.state = 'failed'
  }
  finally {
    if (!disposed) busy.value = false
    controller = undefined
  }
}
async function ask() {
  if (!canAsk.value || composing.value) return
  const question: Question = reactive({ id: ++sequence, query: input.value.trim(), state: 'pending' })
  questions.value.push(question)
  input.value = ''
  await send(question)
}
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
}
onBeforeUnmount(() => {
  disposed = true
  controller?.abort()
  for (const timer of copyTimers.values()) clearTimeout(timer)
})
</script>

<template>
  <section class="space-y-5 border-t border-[var(--border-default)] py-6" aria-label="AI 问答">
    <header class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-lg font-medium">问一问</h2>
        <p class="mt-1 text-sm leading-6 text-[var(--text-muted)]">基于博客已发布文章回答，每次问题独立处理。</p>
      </div>
      <button v-if="questions.length" type="button" :disabled="busy" class="ui-link min-h-11 px-2 text-sm disabled:opacity-50" @click="clear">清空记录</button>
    </header>
    <div v-if="questions.length" class="space-y-7">
      <div v-for="question in questions" :key="question.id" class="min-w-0 space-y-3">
        <p data-testid="chat-question" class="ml-auto w-fit max-w-full whitespace-pre-wrap break-words rounded-2xl bg-[var(--mint-surface)] px-4 py-3 text-sm leading-7 text-[var(--mint-text)]">{{ question.query }}</p>
        <p v-if="question.state === 'pending'" role="status" class="text-sm text-[var(--text-muted)]">正在生成回答…</p>
        <div v-else-if="question.state === 'returned'" class="space-y-3">
          <p role="status" class="text-xs text-[var(--text-muted)]">{{ resultLabel(question.reply) }}</p>
          <p v-if="question.reply?.answer" data-testid="chat-answer" class="whitespace-pre-wrap break-words text-sm leading-7">{{ question.reply.answer }}</p>
          <div v-if="question.reply?.citations?.length" class="space-y-2">
            <h3 class="text-xs font-medium text-[var(--text-muted)]">参考文章</h3>
            <div class="grid min-w-0 gap-2 sm:grid-cols-2">
              <NuxtLink v-for="citation in question.reply.citations" :key="citation.slug" :to="`/posts/${encodeURIComponent(citation.slug)}`" class="ui-panel min-w-0 space-y-1 p-3 hover:border-[var(--accent)]">
                <span class="ui-link block break-words text-sm font-medium">{{ citation.title }}</span>
                <span v-if="citation.summary" class="line-clamp-2 block text-xs leading-6 text-[var(--text-muted)]">{{ citation.summary }}</span>
              </NuxtLink>
            </div>
          </div>
          <button v-if="question.reply?.answer" type="button" class="ui-link min-h-11 text-xs" @click="copy(question)">复制回答</button>
          <p v-if="question.copyNotice" role="status" class="text-xs text-[var(--text-muted)]">{{ question.copyNotice }}</p>
        </div>
        <p v-else role="alert" class="text-sm text-[var(--status-error)]">{{ question.error }}</p>
        <div v-if="retryable(question)" class="flex flex-wrap gap-3 text-sm">
          <button type="button" :disabled="busy" class="ui-link min-h-11 disabled:opacity-50" @click="send(question)">重试问题</button>
          <button type="button" :disabled="busy || !!input.trim()" :title="input.trim() ? '先发送或清空当前草稿，再编辑这个问题' : undefined" class="ui-link min-h-11 disabled:opacity-50" @click="edit(question)">编辑问题</button>
        </div>
      </div>
    </div>
    <form class="ui-panel space-y-2 p-3 focus-within:border-[var(--accent)]" @submit.prevent="ask">
      <label for="blog-question" class="sr-only">你的问题</label>
      <textarea id="blog-question" ref="composer" v-model="input" aria-label="你的问题" aria-describedby="blog-question-help" rows="2" placeholder="比如：多对多关系怎么建模？" class="max-h-60 min-h-16 w-full resize-none rounded-lg border-0 bg-transparent p-2 text-base leading-7 text-[var(--text-primary)] placeholder:text-[var(--text-muted)]" @keydown="onKeydown" @compositionstart="composing = true" @compositionend="composing = false" />
      <div class="flex flex-wrap items-center justify-between gap-2">
        <p id="blog-question-help" class="text-xs leading-6 text-[var(--text-muted)]">Enter 提问，Shift+Enter 换行 · <span class="tabular-nums" :class="inputSize > 500 ? 'text-[var(--status-error)]' : ''">{{ inputSize }}/500 字</span></p>
        <button type="submit" :disabled="!canAsk || composing" class="ui-button px-4 text-sm">提问</button>
      </div>
    </form>
  </section>
</template>
