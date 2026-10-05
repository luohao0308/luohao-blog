<script setup lang="ts">
// Public registration. Creates a READER account and signs the caller in
// immediately (the backend returns the same token pair as login). Field
// rules mirror the backend contract: email, password ≥ 8, display name
// 1-32 characters — client checks give fast feedback, the server stays
// authoritative.
useHead({ title: '注册' })

const route = useRoute()
const { user, errReason, register, ensureSession } = useAuth()

const displayName = ref('')
const email = ref('')
const password = ref('')
const pending = ref(false)
const errorMsg = ref('')

const redirect = computed(() => {
  const target = String(route.query.redirect ?? '/')
  return target.startsWith('/') && !target.startsWith('//') ? target : '/'
})

onMounted(async () => {
  await ensureSession()
  if (user.value)
    navigateTo(redirect.value, { replace: true })
})

async function submit() {
  errorMsg.value = ''
  const name = displayName.value.trim()
  if (!email.value.trim() || !password.value || !name) {
    errorMsg.value = '请填写昵称、邮箱和密码'
    return
  }
  if (name.length > 32) {
    errorMsg.value = '昵称最多 32 个字'
    return
  }
  if (password.value.length < 8) {
    errorMsg.value = '密码至少 8 位'
    return
  }
  pending.value = true
  try {
    await register({ email: email.value.trim(), password: password.value, display_name: name })
    navigateTo(redirect.value, { replace: true })
  }
  catch (err: unknown) {
    const status = (err as { response?: { status?: number } } | null)?.response?.status
    if (status === 409)
      errorMsg.value = '该邮箱已注册，换一个或直接登录'
    else if (status === 429 || errReason(err) === 'AUTH_TOO_MANY_ATTEMPTS')
      errorMsg.value = '注册太频繁，请 5 分钟后再试'
    else if (status === 400)
      errorMsg.value = '信息不符合要求：邮箱格式、密码（至少 8 位）或昵称（1-32 字）有误'
    else
      errorMsg.value = '注册失败，请稍后再试'
  }
  finally {
    pending.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-sm pt-12">
    <div class="space-y-6 rounded-xl border border-slate-200 p-6 dark:border-slate-800">
      <header class="space-y-3">
        <h1 class="text-2xl font-bold tracking-tight">
          注册 <span class="text-blue-600 dark:text-blue-400">luohao</span><span>.blog</span>
        </h1>
        <p class="text-sm text-slate-500 dark:text-slate-400">一分钟注册，登录后即可发表评论（评论先审后显）。</p>
      </header>
      <form class="space-y-4" @submit.prevent="submit">
        <input
          v-model="displayName"
          type="text"
          maxlength="32"
          autocomplete="nickname"
          placeholder="昵称（1-32 个字，评论区展示）"
          class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-blue-500 dark:border-slate-600 dark:bg-slate-900"
        >
        <input
          v-model="email"
          type="email"
          autocomplete="username"
          placeholder="邮箱"
          class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-blue-500 dark:border-slate-600 dark:bg-slate-900"
        >
        <input
          v-model="password"
          type="password"
          autocomplete="new-password"
          placeholder="密码（至少 8 位）"
          class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-blue-500 dark:border-slate-600 dark:bg-slate-900"
        >
        <p v-if="errorMsg" class="text-sm text-red-600 dark:text-red-400" role="alert">
          {{ errorMsg }}
        </p>
        <button
          type="submit"
          :disabled="pending"
          class="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:opacity-50"
        >
          {{ pending ? '注册中…' : '注册并登录' }}
        </button>
      </form>
      <p class="text-center text-sm text-slate-500 dark:text-slate-400">
        已有账号？
        <NuxtLink :to="`/login?redirect=${encodeURIComponent(redirect)}`" class="text-blue-600 hover:underline dark:text-blue-400">
          登录
        </NuxtLink>
      </p>
    </div>
    <p class="mt-6 text-center text-xs text-slate-400 dark:text-slate-500">
      <NuxtLink to="/" class="hover:underline">← 返回首页</NuxtLink>
    </p>
  </div>
</template>
