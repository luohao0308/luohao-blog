<script setup lang="ts">
// Article comment section: the approved list is fetched server-side with the
// page; submissions are client-side and enter moderation, so the UI confirms
// with "等待审核" instead of appending optimistically.
import { COMMENT_NAME_MAX, COMMENT_CONTENT_MAX, type Comment } from '~/composables/useComments'

const props = defineProps<{ slug: string }>()

const { data, error } = await useArticleComments(props.slug)
const comments = computed<Comment[]>(() => data.value?.comments ?? [])

const displayName = ref('')
const content = ref('')
const submitting = ref(false)
const notice = ref('')
const noticeKind = ref<'ok' | 'err'>('ok')

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
    content.value = ''
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
    <p v-else-if="error" class="text-sm text-slate-500 dark:text-slate-400">评论暂时无法加载。</p>
    <p v-else class="text-sm text-slate-500 dark:text-slate-400">还没有评论，来写第一条吧。</p>

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
