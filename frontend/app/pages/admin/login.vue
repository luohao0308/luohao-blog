<script setup lang="ts">
// Admin accounts are provisioned by the seed command. Reader sessions can
// switch accounts here; only admins follow the middleware's return path.
import { NButton, NForm, NFormItem, NInput } from 'naive-ui'

definePageMeta({ layout: 'admin' })

useHead({ title: '登录' })

const route = useRoute()
const { user, errReason, login, ensureSession } = useAuth()

const email = ref('')
const password = ref('')
const pending = ref(false)
const errorMsg = ref('')
const isNonAdmin = computed(() => user.value !== null && user.value.role !== 1)

const redirect = computed(() => {
  const target = String(route.query.redirect ?? '/admin/posts')
  // Only allow in-app paths; blocks //evil.com style open redirects.
  return target.startsWith('/') && !target.startsWith('//') ? target : '/admin/posts'
})

onMounted(async () => {
  await ensureSession()
  if (user.value?.role === 1)
    navigateTo(redirect.value, { replace: true })
})

async function submit() {
  errorMsg.value = ''
  if (!email.value.trim() || !password.value) {
    errorMsg.value = '请输入邮箱和密码'
    return
  }
  pending.value = true
  try {
    await login(email.value.trim(), password.value)
    if (user.value?.role === 1)
      navigateTo(redirect.value, { replace: true })
  }
  catch (err: unknown) {
    const status = (err as { response?: { status?: number } } | null)?.response?.status
    if (status === 429 || errReason(err) === 'AUTH_TOO_MANY_ATTEMPTS')
      errorMsg.value = '尝试次数过多，请 5 分钟后再试'
    else if (status === 401)
      errorMsg.value = '邮箱或密码错误'
    else
      errorMsg.value = '登录失败，请稍后再试'
  }
  finally {
    pending.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-sm pt-16">
    <div class="space-y-6 rounded-xl border border-slate-200 p-6 dark:border-slate-800">
      <header class="space-y-3">
        <p class="text-xs text-[#3c5d85] dark:text-blue-300">Admin / 管理后台</p>
        <h1 class="text-2xl font-bold tracking-tight">
          登录 <span class="text-blue-600 dark:text-blue-400">luohao</span><span class="text-slate-800 dark:text-slate-100">.blog</span>
        </h1>
        <p class="text-sm text-slate-500 dark:text-slate-400">仅站长账号可登录，注册通道不开放。</p>
      </header>
      <p v-if="isNonAdmin" class="text-sm text-amber-700 dark:text-amber-300" role="alert">
        当前账号没有后台管理权限，请使用站长账号登录。
      </p>
      <NForm label-placement="top" @submit.prevent="submit">
        <NFormItem label="邮箱">
          <NInput v-model:value="email" type="text" placeholder="you@example.com" :input-props="{ autocomplete: 'username', type: 'email' }" />
        </NFormItem>
        <NFormItem label="密码">
          <NInput v-model:value="password" type="password" show-password-on="click" placeholder="密码" :input-props="{ autocomplete: 'current-password' }" @keydown.enter="submit" />
        </NFormItem>
        <p v-if="errorMsg" class="mb-3 text-sm text-red-600 dark:text-red-400" role="alert">
          {{ errorMsg }}
        </p>
        <NButton type="primary" attr-type="submit" block :loading="pending">
          登录
        </NButton>
      </NForm>
    </div>
    <p class="mt-6 text-center text-xs text-slate-400 dark:text-slate-500">
      <NuxtLink to="/" class="hover:underline">← 返回前台</NuxtLink>
    </p>
  </div>
</template>
