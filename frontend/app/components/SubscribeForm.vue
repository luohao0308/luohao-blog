<script setup lang="ts">
// Email subscription: registration-only for now. The backend stores the
// lowercased address idempotently; actual sending waits for a mail provider.
const email = ref('')
const submitting = ref(false)
const notice = ref('')
const noticeKind = ref<'ok' | 'err'>('ok')

async function subscribe() {
  notice.value = ''
  const value = email.value.trim()
  if (!value) {
    noticeKind.value = 'err'
    notice.value = '请输入邮箱地址'
    return
  }
  submitting.value = true
  try {
    await $fetch('/api/v1/subscribers/subscribe', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: { email: value },
    })
    noticeKind.value = 'ok'
    notice.value = '已登记订阅，新文章上线时会第一时间通知你。'
    email.value = ''
  }
  catch (err: unknown) {
    noticeKind.value = 'err'
    const status = (err as { response?: { status?: number } } | null)?.response?.status
    notice.value = status === 429
      ? '提交太频繁了，请一小时后再试。'
      : status === 400
        ? '邮箱格式看起来不太对，请检查后重试。'
        : '登记失败，请稍后再试。'
  }
  finally {
    submitting.value = false
  }
}
</script>

<template>
  <section class="max-w-2xl space-y-3 rounded-xl border border-slate-200 p-4 dark:border-slate-800">
    <div>
      <h2 class="text-sm font-medium">订阅更新</h2>
      <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
        登记邮箱，新文章发布时收到通知；也可以用
        <a href="/rss.xml" class="underline underline-offset-2 hover:text-slate-700 dark:hover:text-slate-300">RSS 订阅</a>。
      </p>
    </div>
    <form class="flex flex-col gap-2 sm:flex-row" @submit.prevent="subscribe">
      <label for="subscribe-email" class="sr-only">邮箱地址</label>
      <input
        id="subscribe-email"
        v-model="email"
        type="email"
        autocomplete="email"
        placeholder="you@example.com"
        class="w-full rounded-lg border border-slate-300 bg-transparent px-3 py-2 text-sm outline-none transition focus:border-blue-500 dark:border-slate-700"
      >
      <button
        type="submit"
        :disabled="submitting"
        class="shrink-0 rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-slate-700 disabled:opacity-50 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-200"
      >
        {{ submitting ? '提交中…' : '订阅' }}
      </button>
    </form>
    <p v-if="notice" :class="noticeKind === 'ok' ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'" class="text-sm" role="status">
      {{ notice }}
    </p>
  </section>
</template>
