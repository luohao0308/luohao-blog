<script setup lang="ts">
// Article comment section: the approved list is fetched server-side with the
// page; submissions require a signed-in account (identity comes from the
// access token), so the form waits for the client-side session restore and
// shows a login prompt for anonymous visitors. Submissions enter
// moderation, so the UI confirms with "等待审核" and keeps the pending
// comment visible (in memory only) until a reload picks up the approved
// list.
import { COMMENT_CONTENT_MAX, type Comment } from '~/composables/useComments'

const props = defineProps<{ slug: string }>()

const route = useRoute()
const { user, ensureSession, authFetch } = useAuth()

const { data, error, refresh } = await useArticleComments(props.slug)
const comments = computed<Comment[]>(() => data.value?.comments ?? [])

// The form needs the session state, which only exists client-side; render a
// neutral placeholder until the restore settles.
const sessionReady = ref(false)
onMounted(async () => {
  await ensureSession()
  sessionReady.value = true
})

const content = ref('')
const submitting = ref(false)
const notice = ref('')
const noticeKind = ref<'ok' | 'err'>('ok')
const pending = ref<Array<{ name: string, body: string }>>([])

const loginLink = computed(() => `/login?redirect=${encodeURIComponent(route.path)}`)
const registerLink = computed(() => `/register?redirect=${encodeURIComponent(route.path)}`)

function initialOf(name: string): string {
  return (name || '?').trim().charAt(0).toUpperCase()
}

function avatarSrc(avatarUrl?: string): string {
  return avatarUrl ? assetUrl(avatarUrl) : ''
}

async function submit() {
  notice.value = ''
  const body = content.value.trim()
  if (!body) {
    noticeKind.value = 'err'
    notice.value = '请填写评论内容'
    return
  }
  if (body.length > COMMENT_CONTENT_MAX) {
    noticeKind.value = 'err'
    notice.value = `评论最多 ${COMMENT_CONTENT_MAX} 字`
    return
  }
  submitting.value = true
  try {
    await authFetch('/api/v1/comments', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: { article_slug: props.slug, content: body },
    })
    noticeKind.value = 'ok'
    notice.value = '评论已提交，通过审核后展示。'
    pending.value.push({ name: user.value?.display_name ?? '', body })
    content.value = ''
  }
  catch (err: unknown) {
    noticeKind.value = 'err'
    const status = (err as { response?: { status?: number } } | null)?.response?.status
    notice.value = status === 429
      ? '提交太频繁了，请 5 分钟后再试。'
      : status === 401
        ? '登录状态已失效，请重新登录后再评论。'
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
      <li v-for="c in comments" :key="c.id" class="flex gap-3">
        <img
          v-if="avatarSrc(c.avatar_url)"
          :src="avatarSrc(c.avatar_url)"
          :alt="`${c.display_name} 的头像`"
          class="h-8 w-8 shrink-0 rounded-full object-cover"
        >
        <span
          v-else
          class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-[#3c5d85] text-xs font-semibold text-white dark:bg-blue-600"
          aria-hidden="true"
        >{{ initialOf(c.display_name) }}</span>
        <div class="min-w-0 space-y-1">
          <div class="flex items-baseline gap-3 text-sm">
            <span class="font-medium">{{ c.display_name }}</span>
            <time class="text-xs text-slate-500 dark:text-slate-400">{{ formatDate(c.created_at) }}</time>
          </div>
          <p class="break-words text-sm leading-7 text-slate-700 dark:text-slate-300">{{ c.content }}</p>
        </div>
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

    <!-- 登录引导：评论要求登录身份 -->
    <div v-if="!sessionReady" class="h-24 animate-pulse rounded-md bg-slate-50 dark:bg-slate-800/60" aria-hidden="true" />
    <div v-else-if="!user" class="flex flex-col items-start gap-3 rounded-md border border-dashed border-slate-300 p-4 text-sm dark:border-slate-700 sm:flex-row sm:items-center sm:justify-between">
      <p class="text-slate-600 dark:text-slate-300">
        登录后即可发表评论（评论先审后显），昵称与头像随账号走。
      </p>
      <div class="flex shrink-0 gap-2">
        <NuxtLink :to="loginLink" class="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700">
          登录
        </NuxtLink>
        <NuxtLink :to="registerLink" class="rounded-md border border-slate-300 px-4 py-2 text-sm text-slate-700 transition-colors hover:border-blue-500 hover:text-blue-600 dark:border-slate-600 dark:text-slate-200 dark:hover:border-blue-400 dark:hover:text-blue-300">
          注册
        </NuxtLink>
      </div>
    </div>

    <form v-else class="space-y-3" @submit.prevent="submit">
      <div class="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
        <img
          v-if="avatarSrc(user.avatar_url)"
          :src="avatarSrc(user.avatar_url)"
          :alt="`${user.display_name} 的头像`"
          class="h-7 w-7 rounded-full object-cover"
        >
        <span v-else class="flex h-7 w-7 items-center justify-center rounded-full bg-[#3c5d85] text-xs font-semibold text-white dark:bg-blue-600" aria-hidden="true">{{ initialOf(user.display_name) }}</span>
        <span>以 <span class="font-medium text-slate-900 dark:text-slate-100">{{ user.display_name }}</span> 的身份发表</span>
      </div>
      <textarea
        v-model="content"
        rows="3"
        maxlength="1000"
        placeholder="写下你的评论（审核后展示）"
        class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-blue-500 dark:border-slate-600 dark:bg-slate-900"
      />
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
