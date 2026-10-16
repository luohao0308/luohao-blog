<script setup lang="ts">
useHead({ title: '重置密码' })
const { confirmPasswordReset } = useAccountEmail()
const token = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const pending = ref(false)
const success = ref(false)
const error = ref('')

onMounted(() => { token.value = takeHashToken() })

async function submit() {
  if (pending.value || !token.value) return
  error.value = ''
  if (newPassword.value.length < 8) { error.value = '新密码至少 8 位'; return }
  if (passwordByteLength(newPassword.value) > PASSWORD_MAX_BYTES) { error.value = '新密码过长，请控制在 1024 字节以内'; return }
  if (newPassword.value !== confirmPassword.value) { error.value = '两次输入的新密码不一致'; return }
  pending.value = true
  try {
    await confirmPasswordReset(token.value, newPassword.value)
    token.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    success.value = true
  }
  catch (err) {
    const status = accountEmailStatus(err)
    error.value = status === 429 ? '尝试过于频繁，请稍后重试。' : status === 400 ? '重置链接无效、已过期或已经使用，请重新申请。' : '重置服务暂时不可用，请稍后重试。'
  }
  finally { pending.value = false }
}
</script>

<template>
  <div class="mx-auto max-w-md pt-12">
    <div class="ui-panel space-y-6 p-6">
      <h1 class="text-2xl font-bold">重置密码</h1>
      <div v-if="success" role="status" class="space-y-4"><p class="text-sm text-emerald-600">新密码已生效。其他设备会在当前登录到期后退出；默认设置下最长约 15 分钟。</p><NuxtLink to="/login" class="inline-block ui-link hover:underline">前往登录</NuxtLink></div>
      <form v-else-if="token" class="space-y-4" @submit.prevent="submit">
        <input v-model="newPassword" type="password" autocomplete="new-password" aria-label="新密码（至少 8 位）" placeholder="新密码（至少 8 位）" class="ui-field w-full px-3 py-2">
        <input v-model="confirmPassword" type="password" autocomplete="new-password" aria-label="确认新密码" placeholder="确认新密码" class="ui-field w-full px-3 py-2">
        <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
        <button type="submit" :disabled="pending" class="ui-button w-full px-4 py-2">{{ pending ? '提交中…' : '确认重置' }}</button>
      </form>
      <div v-else role="alert" class="space-y-4"><p class="text-sm text-red-600">重置链接缺失或格式不正确。</p><NuxtLink to="/forgot-password" class="ui-link hover:underline">重新申请找回密码</NuxtLink></div>
      <NuxtLink v-if="!success" to="/login" class="block text-sm text-slate-500 hover:underline">返回登录</NuxtLink>
    </div>
  </div>
</template>
