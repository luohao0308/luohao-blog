<script setup lang="ts">
// Admin-only chrome: slim top bar with management entry points and the
// signed-in account. Follows the site dark mode through the html.dark class.
// naive-ui is themed to the site palette (#3c5d85 brand, 8px radius) so the
// management pages read as part of the same site in both modes.
import { NButton, NConfigProvider, NDialogProvider, NMessageProvider, darkTheme, dateZhCN, zhCN, type GlobalThemeOverrides } from 'naive-ui'

// Keep these literals aligned with main.css semantic colors: Naive UI uses
// color arithmetic internally, so CSS var() values are not safe here.
const lightOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#3c5d85',
    primaryColorHover: '#2d486b',
    primaryColorPressed: '#24405f',
    primaryColorSuppl: '#3c5d85',
    borderRadius: '10px',
    bodyColor: '#ffffff',
    cardColor: '#f8fafc',
    modalColor: '#ffffff',
    popoverColor: '#ffffff',
    inputColor: '#ffffff',
    textColor1: '#0f172a',
    textColor2: '#475569',
    textColor3: '#475569',
    borderColor: '#e2e8f0',
  },
}
const darkOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#93c5fd',
    primaryColorHover: '#bfdbfe',
    primaryColorPressed: '#60a5fa',
    primaryColorSuppl: '#93c5fd',
    borderRadius: '10px',
    bodyColor: '#020617',
    cardColor: '#0f172a',
    modalColor: '#0f172a',
    popoverColor: '#0f172a',
    inputColor: '#0f172a',
    textColor1: '#f1f5f9',
    textColor2: '#94a3b8',
    textColor3: '#94a3b8',
    borderColor: '#334155',
  },
}

const { user, logout, ensureSession } = useAuth()
const { preference, init } = useTheme()

// matchMedia is client-only, so system preference tracking starts empty and
// hydrates on mount alongside the persisted theme choice.
// The client auth guard restores user before hydration, while SSR is anonymous.
// Defer identity-only nodes until mount so the first client tree matches SSR.
const identityReady = ref(false)
const systemDark = ref(false)
let systemQuery: MediaQueryList | undefined
function onSystemThemeChange(event: MediaQueryListEvent) {
  systemDark.value = event.matches
  if (preference.value === 'system') {
    document.documentElement.classList.toggle('dark', event.matches)
    document.documentElement.style.colorScheme = event.matches ? 'dark' : 'light'
  }
}
onBeforeUnmount(() => systemQuery?.removeEventListener('change', onSystemThemeChange))
onMounted(() => {
  identityReady.value = true
  init()
  systemQuery = window.matchMedia('(prefers-color-scheme: dark)')
  systemDark.value = systemQuery.matches
  systemQuery.addEventListener('change', onSystemThemeChange)
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
  <NConfigProvider :theme="isDark ? darkTheme : null" :theme-overrides="isDark ? darkOverrides : lightOverrides" :locale="zhCN" :date-locale="dateZhCN">
    <NMessageProvider>
      <NDialogProvider>
        <div class="flex min-h-screen flex-col site-shell antialiased transition-colors duration-[var(--motion-standard)]">
          <a href="#main-content" class="skip-link">跳到主要内容</a>
          <header class="top-0 z-40 border-b site-header backdrop-blur md:sticky">
            <div class="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-6 py-3">
              <div class="flex flex-wrap items-center gap-x-6 gap-y-2">
                <NuxtLink to="/" class="text-lg font-bold tracking-tight">
                  luohao<span class="text-[var(--accent)]">.blog</span>
                </NuxtLink>
                <span class="text-sm text-[var(--text-muted)] tabular-nums">管理后台</span>
                <NuxtLink to="/admin/posts" class="text-sm text-[var(--text-muted)] hover:text-[var(--text-primary)]">
                  文章管理
                </NuxtLink>
                <NuxtLink to="/admin/categories" class="text-sm text-[var(--text-muted)] hover:text-[var(--text-primary)]">
                  分类管理
                </NuxtLink>
                <NuxtLink to="/admin/projects" class="text-sm text-[var(--text-muted)] hover:text-[var(--text-primary)]">
                  项目管理
                </NuxtLink>
                <NuxtLink to="/admin/about" class="text-sm text-[var(--text-muted)] hover:text-[var(--text-primary)]">
                  关于页管理
                </NuxtLink>
                <NuxtLink to="/admin/comments" class="text-sm text-[var(--text-muted)] hover:text-[var(--text-primary)]">
                  评论管理
                </NuxtLink>
                <NuxtLink to="/admin/wechat-sync" class="text-sm text-[var(--text-muted)] hover:text-[var(--text-primary)]">公众号同步</NuxtLink>
                <NuxtLink to="/admin/subscribers" class="text-sm text-[var(--text-muted)] hover:text-[var(--text-primary)]">
                  订阅管理
                </NuxtLink>
              </div>
              <div class="flex flex-wrap items-center gap-3 text-sm">
                <NuxtLink to="/settings" class="flex items-center gap-2 text-[var(--text-muted)] hover:text-[var(--accent)]">
                  <img v-if="identityReady && user?.avatar_url" :src="assetUrl(user.avatar_url)" alt="账号头像" class="h-8 w-8 rounded-full object-cover">
                  <span>头像与个人设置</span>
                </NuxtLink>
                <span v-if="identityReady && user" class="text-[var(--text-muted)]">{{ user.display_name }}</span>
                <NuxtLink to="/settings#password" class="text-[var(--text-muted)] hover:text-[var(--accent)]">
                  修改密码
                </NuxtLink>
                <NButton style="min-height: 44px" size="small" quaternary @click="onLogout">
                  退出
                </NButton>
              </div>
            </div>
          </header>

          <main id="main-content" tabindex="-1" class="site-main mx-auto min-w-0 w-full max-w-6xl flex-1 px-6 py-8">
            <slot />
          </main>
        </div>
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>
