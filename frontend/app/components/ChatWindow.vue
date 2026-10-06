<script setup lang="ts">
// Blog chat: one question at a time against the published-article corpus,
// answered server-side by retrieval-augmented generation with citations.
// The answer is plain text (the API prompt forbids Markdown), citations link
// back into the site.
interface CitedArticle {
  slug: string
  title: string
  summary: string
}

interface Turn {
  role: 'user' | 'assistant'
  text: string
  citations?: CitedArticle[]
}

const turns = ref<Turn[]>([])
const input = ref('')
const loading = ref(false)
const notice = ref('')

async function ask() {
  const query = input.value.trim()
  if (!query || loading.value)
    return
  notice.value = ''
  turns.value.push({ role: 'user', text: query })
  input.value = ''
  loading.value = true
  try {
    const reply = await $fetch<{ answer: string, citations?: CitedArticle[] }>('/api/v1/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: { query },
    })
    turns.value.push({ role: 'assistant', text: reply.answer, citations: reply.citations ?? [] })
  }
  catch (err: unknown) {
    const status = (err as { response?: { status?: number } } | null)?.response?.status
    notice.value = status === 429
      ? '提问太频繁了，请 5 分钟后再试。'
      : status === 400
        ? '问题不符合要求（不超过 500 字）。'
        : status === 503
          ? 'AI 问答还没有配置好，请稍后再试。'
          : '回答生成失败，请稍后再试。'
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="space-y-4 rounded-xl border border-slate-200 p-5 dark:border-slate-800" aria-label="AI 问答">
    <div>
      <h2 class="text-lg font-medium">问一问</h2>
      <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
        基于博客已发布的文章回答问题，回答会附上出处。
      </p>
    </div>

    <div v-if="turns.length" class="space-y-4">
      <div v-for="(turn, i) in turns" :key="i" :class="turn.role === 'user' ? 'text-right' : ''">
        <div
          class="inline-block max-w-full whitespace-pre-wrap rounded-xl px-4 py-3 text-left text-sm leading-6"
          :class="turn.role === 'user'
            ? 'bg-[#e7eee9] text-[#365f53] dark:bg-emerald-950 dark:text-emerald-100'
            : 'bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-100'"
        >{{ turn.text }}</div>
        <div v-if="turn.citations?.length" class="mt-2 space-x-2 text-xs text-slate-500 dark:text-slate-400">
          <span>出处：</span>
          <NuxtLink
            v-for="c in turn.citations"
            :key="c.slug"
            :to="`/posts/${encodeURIComponent(c.slug)}`"
            class="text-[#3c5d85] hover:underline dark:text-blue-300"
          >{{ c.title }}</NuxtLink>
        </div>
      </div>
    </div>

    <p v-if="notice" class="text-sm text-rose-600 dark:text-rose-400">{{ notice }}</p>

    <form class="flex gap-2" @submit.prevent="ask">
      <input
        v-model="input"
        type="text"
        maxlength="500"
        placeholder="比如：多对多关系怎么建模？"
        class="w-full rounded-xl border border-slate-300 bg-transparent px-3 py-2 text-sm leading-6 outline-none focus:border-[#3c5d85] dark:border-slate-700 dark:focus:border-blue-400"
        :disabled="loading"
      >
      <button
        type="submit"
        :disabled="loading || !input.trim()"
        class="shrink-0 rounded-full bg-[#3c5d85] px-4 py-2 text-sm text-white disabled:opacity-50 dark:bg-blue-600"
      >
        {{ loading ? '思考中…' : '提问' }}
      </button>
    </form>
  </section>
</template>
