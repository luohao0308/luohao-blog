<script setup lang="ts">
// Public login page. Same session machinery as the admin login (useAuth),
// but open to every account: readers land here from the header 登录 button
// and return to the ?redirect= target (default home). Comment gating (S4)
// will deep-link here with ?redirect=<article>.
useHead({ title: '登录' })

const route = useRoute()
const { user, errReason, login, ensureSession } = useAuth()

const email = ref('')
const password = ref('')
const pending = ref(false)
const errorMsg = ref('')

const redirect = computed(() => {
  const target = String(route.query.redirect ?? '/')
  // Only allow in-app paths; blocks //evil.com style open redirects.
  return target.startsWith('/') && !target.startsWith('//') ? target : '/'
})

onMounted(async () => {
  await ensureSession()
  if (user.value)
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
  <div class="mx-auto max-w-sm pt-12">
    <div class="space-y-6 rounded-xl border border-slate-200 p-6 dark:border-slate-800">
      <header class="space-y-3">
        <h1 class="text-2xl font-bold tracking-tight">
          登录 <span class="text-blue-600 dark:text-blue-400">luohao</span><span>.blog</span>
        </h1>
        <p class="text-sm text-slate-500 dark:text-slate-400">登录后可以评论、收藏身份互通，头像昵称随账号走。</p>
      </header>
      <form class="space-y-4" @submit.prevent="submit">
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
          autocomplete="current-password"
          placeholder="密码"
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
          {{ pending ? '登录中…' : '登录' }}
        </button>
      </form>
      <p class="text-center text-sm text-slate-500 dark:text-slate-400">
        还没有账号？
        <NuxtLink :to="`/register?redirect=${encodeURIComponent(redirect)}`" class="text-blue-600 hover:underline dark:text-blue-400">
          注册
        </NuxtLink>
      </p>
    </div>
    <p class="mt-6 text-center text-xs text-slate-400 dark:text-slate-500">
      <NuxtLink to="/" class="hover:underline">← 返回首页</NuxtLink>
    </p>
  </div>
</template>
