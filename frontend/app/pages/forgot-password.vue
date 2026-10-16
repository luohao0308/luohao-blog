<script setup lang="ts">
useHead({ title: '找回密码' })
const { requestPasswordReset } = useAccountEmail()
const email = ref('')
const pending = ref(false)
const notice = ref('')
const error = ref('')

async function submit() {
  if (pending.value) return
  error.value = ''
  notice.value = ''
  if (!email.value.trim()) {
    error.value = '请输入邮箱地址'
    return
  }
  pending.value = true
  try {
    await requestPasswordReset(email.value.trim())
    notice.value = '请求已接收，若邮箱已注册且投递可用将收到链接；未收到可稍后重试。'
  }
  catch (err) {
    const status = accountEmailStatus(err)
    error.value = status === 429 ? '请求过于频繁，请稍后重试。' : status === 503 ? '邮件服务暂时不可用，请稍后重试。' : '请求暂时无法处理，请稍后重试。'
  }
  finally { pending.value = false }
}
</script>

<template>
  <div class="mx-auto max-w-md pt-12">
    <div class="ui-panel space-y-6 p-6">
      <header class="space-y-2"><h1 class="text-2xl font-bold">找回密码</h1><p class="text-sm text-slate-500">使用注册邮箱申请重置链接。</p></header>
      <form class="space-y-4" @submit.prevent="submit">
        <input v-model="email" type="email" autocomplete="email" aria-label="邮箱" placeholder="邮箱" class="ui-field w-full px-3 py-2 text-sm">
        <p v-if="notice" role="status" class="text-sm text-emerald-600 dark:text-emerald-400">{{ notice }}</p>
        <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
        <button type="submit" :disabled="pending" class="ui-button w-full px-4 py-2 text-sm font-medium">{{ pending ? '提交中…' : '提交找回请求' }}</button>
      </form>
      <NuxtLink to="/login" class="block text-center text-sm ui-link hover:underline">返回登录</NuxtLink>
    </div>
  </div>
</template>
