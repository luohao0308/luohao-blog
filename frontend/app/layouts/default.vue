<script setup lang="ts">
// Site skeleton: sticky translucent nav with the consolidated sitemap (the
// content-taxonomy pages collapse under the 文章 dropdown) and the identity
// area (theme toggle + login/avatar). The theme class is applied before
// hydration by the inline script registered in app.vue; the login session is
// restored client-side by ensureSession, so SSR renders logged-out and the
// header swaps to the avatar after hydration.
const { init } = useTheme()
const { ensureSession } = useAuth()

onMounted(() => {
  init()
  // Restores an existing refresh-cookie session (admin or reader) for the
  // public header. Anonymous visitors pay one cheap 401 probe per page load.
  ensureSession()
})

// Pages can opt into a wider container via definePageMeta({ wide: true });
// the homepage uses it for its two-column layout.
const route = useRoute()
const wide = computed(() => route.meta.wide === true)

// Desktop keeps five top-level entries; mobile stays a flat list so no
// hamburger menu is needed.
const primaryLinks = [
  { to: '/', label: '首页' },
]
const tailLinks = [
  { to: '/collections', label: '收藏' },
  { to: '/projects', label: '作品集' },
  { to: '/about', label: '关于' },
]
const mobileLinks = [
  { to: '/', label: '首页' },
  { to: '/posts', label: '文章' },
  { to: '/categories', label: '分类' },
  { to: '/tags', label: '标签' },
  { to: '/archives', label: '归档' },
  { to: '/collections', label: '收藏' },
  { to: '/projects', label: '作品集' },
  { to: '/about', label: '关于' },
]
</script>

<template>
  <div class="flex min-h-screen flex-col bg-white text-slate-900 antialiased transition-colors dark:bg-slate-950 dark:text-slate-100">
    <header class="sticky top-0 z-40 border-b border-slate-200/70 bg-white/80 backdrop-blur dark:border-slate-800 dark:bg-slate-950/80">
      <div class="mx-auto flex max-w-4xl items-center justify-between px-6 py-3">
        <NuxtLink to="/" class="text-lg font-bold tracking-tight">
          luohao<span class="text-blue-600 dark:text-blue-400">.blog</span>
        </NuxtLink>
        <nav class="hidden items-center gap-6 text-sm sm:flex">
          <NuxtLink
            v-for="link in primaryLinks"
            :key="link.to"
            :to="link.to"
            class="text-slate-600 transition-colors hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100"
          >
            {{ link.label }}
          </NuxtLink>
          <NavDropdown />
          <NuxtLink
            v-for="link in tailLinks"
            :key="link.to"
            :to="link.to"
            class="text-slate-600 transition-colors hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100"
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
      <nav class="flex flex-wrap items-center gap-x-5 gap-y-1 border-t border-slate-100 px-6 py-2 text-sm sm:hidden dark:border-slate-800/70">
        <NuxtLink
          v-for="link in mobileLinks"
          :key="link.to"
          :to="link.to"
          class="text-slate-600 dark:text-slate-400"
        >
          {{ link.label }}
        </NuxtLink>
      </nav>
    </header>

    <main class="mx-auto w-full flex-1 px-6 py-10" :class="wide ? 'max-w-6xl' : 'max-w-4xl'">
      <slot />
    </main>

    <footer class="border-t border-slate-200 py-6 text-center text-sm text-slate-500 dark:border-slate-800">
      <p>
        © luohao ·
        <a
          href="https://github.com/luohao0308/luohao-blog"
          class="hover:text-slate-700 dark:hover:text-slate-300"
          target="_blank"
          rel="noopener"
        >luohao-blog</a>
        · Go &amp; Nuxt 驱动
      </p>
    </footer>
  </div>
</template>
