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
const passwordChanged = computed(() => route.query.password_changed === '1')

const redirect = computed(() => {
  const target = String(route.query.redirect ?? '/')
  // Only allow in-app paths; blocks //evil.com style open redirects.
  return target.startsWith('/') && !target.startsWith('//') ? target : '/'
})

onMounted(async () => {
  await ensureSession()
  if (user.value && route.query.switch !== '1')
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
    <div class="ui-panel space-y-6 p-6">
      <header class="space-y-3">
        <h1 class="text-2xl font-bold tracking-tight">
          登录 <span class="text-[var(--accent)]">luohao</span><span>.blog</span>
        </h1>
        <p class="text-sm text-slate-500 dark:text-slate-400">登录后可以评论、收藏身份互通，头像昵称随账号走。</p>
      </header>
      <form class="space-y-4" @submit.prevent="submit">
        <p v-if="passwordChanged" role="status" class="text-sm text-emerald-600 dark:text-emerald-400">密码已修改，请使用新密码登录。</p>
        <input
          v-model="email"
          type="email"
          autocomplete="username"
          aria-label="邮箱" placeholder="邮箱"
          class="ui-field w-full px-3 py-2 text-sm"
        >
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          aria-label="密码" placeholder="密码"
          class="ui-field w-full px-3 py-2 text-sm"
        >
        <p v-if="errorMsg" class="text-sm text-red-600 dark:text-red-400" role="alert">
          {{ errorMsg }}
        </p>
        <button
          type="submit"
          :disabled="pending"
          class="ui-button w-full px-4 py-2 text-sm font-medium"
        >
          {{ pending ? '登录中…' : '登录' }}
        </button>
      </form>
      <p class="text-center text-sm"><NuxtLink to="/forgot-password" class="ui-link hover:underline">忘记密码？</NuxtLink></p>
      <p class="text-center text-sm text-slate-500 dark:text-slate-400">
        还没有账号？
        <NuxtLink :to="`/register?redirect=${encodeURIComponent(redirect)}`" class="ui-link hover:underline">
          注册
        </NuxtLink>
      </p>
    </div>
    <p class="mt-6 text-center text-xs text-slate-400 dark:text-slate-500">
      <NuxtLink to="/" class="hover:underline">← 返回首页</NuxtLink>
    </p>
  </div>
</template>
