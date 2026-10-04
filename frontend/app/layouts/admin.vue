<script setup lang="ts">
// Admin-only chrome: slim top bar with management entry points and the
// signed-in account. Follows the site dark mode through the html.dark class.
import { NButton, NConfigProvider, NDialogProvider, NMessageProvider, darkTheme, dateZhCN, zhCN } from 'naive-ui'

const { user, logout, ensureSession } = useAuth()
const { preference, init } = useTheme()

// matchMedia is client-only, so system preference tracking starts empty and
// hydrates on mount alongside the persisted theme choice.
const systemDark = ref(false)
onMounted(() => {
  init()
  systemDark.value = window.matchMedia('(prefers-color-scheme: dark)').matches
  ensureSession()
})

const isDark = computed(() =>
  preference.value === 'dark' || (preference.value === 'system' && systemDark.value),
)

// No useMessage here: this layout is the provider itself, and naive-ui
// message APIs only resolve inside its descendants.
async function onLogout() {
  await logout()
  navigateTo('/admin/login', { replace: true })
}
</script>

<template>
  <NConfigProvider :theme="isDark ? darkTheme : null" :locale="zhCN" :date-locale="dateZhCN">
    <NMessageProvider>
      <NDialogProvider>
        <div class="flex min-h-screen flex-col bg-slate-50 text-slate-900 antialiased transition-colors dark:bg-slate-950 dark:text-slate-100">
          <header class="sticky top-0 z-40 border-b border-slate-200/70 bg-white/80 backdrop-blur dark:border-slate-800 dark:bg-slate-950/80">
            <div class="mx-auto flex max-w-6xl items-center justify-between px-6 py-3">
              <div class="flex items-center gap-6">
                <NuxtLink to="/" class="text-lg font-bold tracking-tight">
                  luohao<span class="text-blue-600 dark:text-blue-400">.blog</span>
                </NuxtLink>
                <span class="text-sm text-slate-500 dark:text-slate-400">管理后台</span>
                <NuxtLink to="/admin/posts" class="text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100">
                  文章管理
                </NuxtLink>
                <NuxtLink to="/admin/categories" class="text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100">
                  分类管理
                </NuxtLink>
                <NuxtLink to="/admin/comments" class="text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100">
                  评论管理
                </NuxtLink>
              </div>
              <div class="flex items-center gap-3 text-sm">
                <span v-if="user" class="text-slate-600 dark:text-slate-300">{{ user.display_name }}</span>
                <NButton size="small" quaternary @click="onLogout">
                  退出
                </NButton>
              </div>
            </div>
          </header>

          <main class="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
            <slot />
          </main>
        </div>
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>
