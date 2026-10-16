<script setup lang="ts">
import type { AuthUser } from '~/composables/useAuth'
useHead({ title: '验证邮箱' })

const { user, setUser, authFetch } = useAuth()
const { confirmVerification } = useAccountEmail()
const token = ref('')
const pending = ref(false)
const success = ref(false)
const error = ref('')

onMounted(() => { token.value = takeHashToken() })

async function confirm() {
  if (pending.value || !token.value) return
  pending.value = true
  error.value = ''
  try {
    await confirmVerification(token.value)
    token.value = ''
    success.value = true
    // A public verification token may belong to a different email than the
    // account currently signed in on this browser. Re-read that account's
    // authoritative state instead of attributing the token to it.
    if (user.value) {
      try { setUser(await authFetch<AuthUser>('/api/v1/auth/me')) }
      catch { /* confirmation succeeded; settings can retry session refresh */ }
    }
  }
  catch (err) {
    const status = accountEmailStatus(err)
    error.value = status === 429 ? '尝试过于频繁，请稍后重试。' : status === 400 ? '验证链接无效、已过期或已经使用。' : '验证服务暂时不可用，请稍后重试。'
  }
  finally { pending.value = false }
}
</script>

<template>
  <div class="mx-auto max-w-md pt-12">
    <div class="ui-panel space-y-6 p-6">
      <h1 class="text-2xl font-bold">验证邮箱</h1>
      <div v-if="success" role="status" class="space-y-4"><p class="text-sm text-emerald-600">邮箱验证成功。</p><NuxtLink :to="user ? '/settings' : '/login'" class="ui-link hover:underline">继续</NuxtLink></div>
      <div v-else-if="token" class="space-y-4"><p class="text-sm text-slate-600 dark:text-slate-400">确认此邮箱属于你。验证不会影响现有登录状态。</p><p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p><button type="button" :disabled="pending" class="ui-button px-5 py-2 text-sm" @click="confirm">{{ pending ? '验证中…' : '确认验证' }}</button></div>
      <div v-else role="alert" class="space-y-4"><p class="text-sm text-red-600">验证链接缺失或格式不正确。</p><NuxtLink :to="user ? '/settings' : '/login'" class="ui-link hover:underline">返回并重新申请</NuxtLink></div>
    </div>
  </div>
</template>
