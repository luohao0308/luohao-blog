<script setup lang="ts">
// Site skeleton: sticky translucent nav with a flat sitemap (no dropdowns —
// 分类/标签/归档 are reachable from the 文章 page) and the identity area
// (theme toggle + login/avatar). The theme class is applied before hydration
// by the inline script registered in app.vue; the login session is restored
// client-side by ensureSession, so SSR renders logged-out and the header
// swaps to the avatar after hydration.
const { init } = useTheme()
const { ensureSession } = useAuth()
const { public: publicConfig } = useRuntimeConfig()

onMounted(() => {
  init()
  // Restores an existing refresh-cookie session (admin or reader) for the
  // public header. Anonymous visitors pay one cheap 401 probe per page load.
  ensureSession()
})

// Pages can opt into a wider container via definePageMeta({ wide: true }).
const route = useRoute()
const wide = computed(() => route.meta.wide === true)

// Flat top-level nav; 收藏 lives in the footer (it is a local-browser
// feature, not a primary destination). Mobile keeps the taxonomy pages as a
// flat wrap list so no hamburger menu is needed.
const navLinks = [
  { to: '/', label: '首页' },
  { to: '/posts', label: '文章' },
  { to: '/projects', label: '作品集' },
  { to: '/about', label: '关于' },
]
const mobileLinks = [
  { to: '/', label: '首页' },
  { to: '/posts', label: '文章' },
  { to: '/categories', label: '分类' },
  { to: '/tags', label: '标签' },
  { to: '/archives', label: '归档' },
  { to: '/projects', label: '作品集' },
  { to: '/about', label: '关于' },
]

function isActive(to: string) {
  if (to === '/')
    return route.path === '/'
  return route.path === to || route.path.startsWith(`${to}/`)
}
</script>

<template>
  <div class="flex min-h-screen flex-col site-shell antialiased transition-colors duration-[var(--motion-standard)]">
    <a href="#main-content" class="skip-link">跳到主要内容</a>
    <header class="sticky top-0 z-40 border-b site-header backdrop-blur">
      <div class="mx-auto flex max-w-6xl items-center justify-between px-6 py-3">
        <NuxtLink to="/" class="text-lg font-bold tracking-tight">
          luohao<span class="text-[var(--accent)]">.blog</span>
        </NuxtLink>
        <nav class="hidden items-center gap-6 text-sm sm:flex">
          <NuxtLink
            v-for="link in navLinks"
            :key="link.to"
            :to="link.to"
            class="rounded-full px-3 py-1.5 transition-colors hover:bg-[var(--surface-hover)]"
            :class="isActive(link.to)
              ? 'font-medium text-[var(--text-primary)]'
              : 'text-[var(--text-muted)]'"
            :aria-current="isActive(link.to) ? 'page' : undefined"
          >
            {{ link.label }}
          </NuxtLink>
        </nav>
        <div class="flex items-center gap-2">
          <ThemeToggle />
          <UserMenu />
        </div>
      </div>
      <!-- mobile nav: flat destinations, no hamburger -->
      <nav class="flex flex-wrap items-center gap-x-5 gap-y-1 border-t border-[var(--border-default)] px-6 py-2 text-sm sm:hidden">
        <NuxtLink
          v-for="link in mobileLinks"
          :key="link.to"
          :to="link.to"
          class="transition-colors"
          :class="isActive(link.to)
            ? 'font-medium text-[var(--text-primary)]'
            : 'text-[var(--text-muted)]'"
          :aria-current="isActive(link.to) ? 'page' : undefined"
        >
          {{ link.label }}
        </NuxtLink>
      </nav>
    </header>

    <main id="main-content" tabindex="-1" class="site-main mx-auto w-full flex-1 px-6 py-10" :class="wide ? 'max-w-6xl' : 'max-w-4xl'">
      <slot />
    </main>

    <footer class="border-t border-[var(--border-default)] py-6 text-center text-sm text-[var(--text-muted)]">
      <nav class="mb-2 flex items-center justify-center gap-5" aria-label="页脚导航">
        <NuxtLink to="/collections" class="transition-colors hover:text-[var(--accent)]">收藏</NuxtLink>
        <a
          href="https://github.com/luohao0308/luohao-blog"
          class="transition-colors hover:text-[var(--accent)]"
          target="_blank"
          rel="noopener"
        >GitHub</a>
      </nav>
      <p>© luohao · Go &amp; Nuxt 驱动</p>
      <a
        v-if="publicConfig.icpNumber"
        href="https://beian.miit.gov.cn/"
        class="mt-2 inline-block transition-colors hover:text-[var(--accent)]"
        target="_blank"
        rel="noopener"
      >{{ publicConfig.icpNumber }}</a>
    </footer>
  </div>
</template>
