<script setup lang="ts">
// Site skeleton: sticky translucent nav with the five sitemap entries and the
// theme toggle; footer with copyright. The theme class is applied before
// hydration by the inline script registered in app.vue.
const { init } = useTheme()

onMounted(() => {
  init()
})

const links = [
  { to: '/', label: '首页' },
  { to: '/posts', label: '文章' },
  { to: '/tags', label: '标签' },
  { to: '/projects', label: '作品集' },
  { to: '/archives', label: '归档' },
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
            v-for="link in links"
            :key="link.to"
            :to="link.to"
            class="text-slate-600 transition-colors hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100"
          >
            {{ link.label }}
          </NuxtLink>
        </nav>
        <div class="flex items-center gap-1">
          <ThemeToggle />
        </div>
      </div>
      <!-- mobile nav -->
      <nav class="flex items-center gap-5 border-t border-slate-100 px-6 py-2 text-sm sm:hidden dark:border-slate-800/70">
        <NuxtLink
          v-for="link in links"
          :key="link.to"
          :to="link.to"
          class="text-slate-600 dark:text-slate-400"
        >
          {{ link.label }}
        </NuxtLink>
      </nav>
    </header>

    <main class="mx-auto w-full max-w-4xl flex-1 px-6 py-10">
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
