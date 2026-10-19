<script setup lang="ts">
const notice = ref('')
async function copyAccount() {
  try {
    await navigator.clipboard.writeText('makerhao1')
    notice.value = '微信号已复制，打开微信搜索并关注。'
  }
  catch {
    notice.value = '请在微信中搜索 makerhao1。'
  }
}
</script>

<template>
  <section class="mx-auto flex max-w-2xl flex-col gap-5 rounded-xl border border-slate-200 bg-slate-50 p-6 shadow-sm sm:flex-row sm:items-center sm:justify-between dark:border-slate-800 dark:bg-slate-900">
    <div class="space-y-2">
      <h2 class="font-semibold">在微信里找到我</h2>
      <p class="text-sm leading-6 text-slate-600 dark:text-slate-400">关注公众号「makerhao」，交流全栈开发与 AI Agent 实践。</p>
      <p class="text-xs text-slate-500 dark:text-slate-400">微信搜索：makerhao1</p>
      <p v-if="notice" role="status" class="text-sm text-blue-600 dark:text-blue-300">{{ notice }}</p>
    </div>
    <button type="button" class="shrink-0 rounded-full bg-slate-900 px-5 py-2.5 text-sm text-white hover:bg-slate-700 focus-visible:outline-2 focus-visible:outline-offset-4 dark:bg-white dark:text-slate-900" @click="copyAccount">复制公众号微信号</button>
  </section>
</template>
