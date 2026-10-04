<script setup lang="ts">
// Article comment section: the approved list is fetched server-side with the
// page; submissions are client-side and enter moderation, so the UI confirms
// with "等待审核" and keeps the pending comment visible (in memory only)
// until a reload picks up the approved list.
import { COMMENT_NAME_MAX, COMMENT_CONTENT_MAX, type Comment } from '~/composables/useComments'

const props = defineProps<{ slug: string }>()

const { data, error, refresh } = await useArticleComments(props.slug)
const comments = computed<Comment[]>(() => data.value?.comments ?? [])

// Nickname persists in this browser only, prefilling the next comment.
const displayName = ref('')
const content = ref('')
const submitting = ref(false)
const notice = ref('')
const noticeKind = ref<'ok' | 'err'>('ok')
const pending = ref<Array<{ name: string, body: string }>>([])

const COMMENT_NAME_KEY = 'blog:comment-name'

onMounted(() => {
  displayName.value = localStorage.getItem(COMMENT_NAME_KEY) ?? ''
})

async function submit() {
  notice.value = ''
  const name = displayName.value.trim()
  const body = content.value.trim()
  if (!name || !body) {
    noticeKind.value = 'err'
    notice.value = '请填写昵称和评论内容'
    return
  }
  if (name.length > COMMENT_NAME_MAX || body.length > COMMENT_CONTENT_MAX) {
    noticeKind.value = 'err'
    notice.value = `昵称最多 ${COMMENT_NAME_MAX} 字、评论最多 ${COMMENT_CONTENT_MAX} 字`
    return
  }
  submitting.value = true
  try {
    await $fetch('/api/v1/comments', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: { article_slug: props.slug, display_name: name, content: body },
    })
    noticeKind.value = 'ok'
    notice.value = '评论已提交，通过审核后展示。'
    pending.value.push({ name, body })
    content.value = ''
    localStorage.setItem(COMMENT_NAME_KEY, name)
  }
  catch (err: unknown) {
    noticeKind.value = 'err'
    const status = (err as { response?: { status?: number } } | null)?.response?.status
    notice.value = status === 429
      ? '提交太频繁了，请 5 分钟后再试。'
      : status === 400
        ? '评论内容不符合要求，请检查后重试。'
        : '提交失败，请稍后再试。'
  }
  finally {
    submitting.value = false
  }
}

function formatDate(ts?: import('~/composables/useArticles').ArticleTimestamp): string {
  const d = new Date(typeof ts === 'string' ? ts : Number((ts as { seconds?: string })?.seconds ?? 0) * 1000)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleDateString('zh-CN', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' })
}
</script>

<template>
  <section class="space-y-6 border-t border-slate-200 pt-8 dark:border-slate-800" aria-label="评论区">
    <h2 class="text-lg font-medium">
      评论 <span v-if="comments.length" class="text-sm text-slate-500 dark:text-slate-400">{{ comments.length }}</span>
    </h2>

    <ul v-if="comments.length" class="space-y-5">
      <li v-for="c in comments" :key="c.id" class="space-y-1">
        <div class="flex items-baseline gap-3 text-sm">
          <span class="font-medium">{{ c.display_name }}</span>
          <time class="text-xs text-slate-500 dark:text-slate-400">{{ formatDate(c.created_at) }}</time>
        </div>
        <p class="break-words text-sm leading-7 text-slate-700 dark:text-slate-300">{{ c.content }}</p>
      </li>
    </ul>
    <div v-else-if="error" class="flex items-center gap-3 text-sm text-slate-500 dark:text-slate-400">
      <span>评论暂时无法加载。</span>
      <button type="button" class="text-[#3c5d85] underline dark:text-blue-300" @click="refresh()">重新加载</button>
    </div>
    <p v-else class="text-sm text-slate-500 dark:text-slate-400">还没有评论，来写第一条吧。</p>

    <ul v-if="pending.length" class="space-y-3" aria-label="我的待审核评论">
      <li
        v-for="(entry, index) in pending"
        :key="index"
        class="space-y-1 rounded-md border border-dashed border-slate-300 p-3 dark:border-slate-700"
      >
        <div class="flex items-baseline gap-3 text-sm">
          <span class="font-medium text-slate-600 dark:text-slate-300">{{ entry.name }}</span>
          <span class="rounded bg-amber-100 px-1.5 py-0.5 text-xs text-amber-700 dark:bg-amber-950 dark:text-amber-300">等待审核</span>
        </div>
        <p class="break-words text-sm leading-7 text-slate-500 dark:text-slate-400">{{ entry.body }}</p>
      </li>
    </ul>

    <form class="space-y-3" @submit.prevent="submit">
      <div class="grid gap-3 sm:grid-cols-[200px_1fr]">
        <input
          v-model="displayName"
          type="text"
          maxlength="32"
          placeholder="昵称"
          class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-blue-500 dark:border-slate-600 dark:bg-slate-900"
        >
        <textarea
          v-model="content"
          rows="3"
          maxlength="1000"
          placeholder="写下你的评论（审核后展示）"
          class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-blue-500 dark:border-slate-600 dark:bg-slate-900"
        />
      </div>
      <p class="text-right text-xs text-slate-400 dark:text-slate-500">
        {{ content.length }} / {{ COMMENT_CONTENT_MAX }}
      </p>
      <p v-if="notice" :class="noticeKind === 'ok' ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'" class="text-sm" role="status">
        {{ notice }}
      </p>
      <button
        type="submit"
        :disabled="submitting"
        class="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:opacity-50"
      >
        {{ submitting ? '提交中…' : '发表评论' }}
      </button>
    </form>
  </section>
</template>
