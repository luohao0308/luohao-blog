import tailwindcss from '@tailwindcss/vite'

export default defineNuxtConfig({
  compatibilityDate: '2026-09-29',
  modules: ['@nuxt/eslint', 'nuxtjs-naive-ui'],
  css: ['~/assets/css/main.css', '~/assets/css/article.css'],
  vite: {
    plugins: [tailwindcss()],
  },
  devtools: { enabled: false },
  runtimeConfig: {
    // Backend Kratos HTTP server; override with NUXT_BACKEND_BASE in
    // deployment (docker compose service name or Caddy upstream).
    backendBase: 'http://127.0.0.1:8000',
  },
  app: {
    head: {
      link: [
        // Feed discovery: readers and aggregators pick the feed up from the
        // document head without a visible link.
        { rel: 'alternate', type: 'application/rss+xml', title: 'luohao.blog', href: '/rss.xml' },
      ],
    },
  },
  typescript: {
    strict: true,
    typeCheck: false,
  },
})
