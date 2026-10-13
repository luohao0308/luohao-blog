<script setup lang="ts">
// The inline script applies the persisted theme class before first paint so
// SSR pages never flash the wrong color scheme.
const themeScript = `(function(){try{var p=localStorage.getItem('theme')||'system';var d=p==='dark'||(p==='system'&&window.matchMedia('(prefers-color-scheme: dark)').matches);document.documentElement.classList.toggle('dark',d);document.documentElement.style.colorScheme=d?'dark':'light';}catch(e){}})();`

const route = useRoute()
const config = useRuntimeConfig()
const siteOrigin = config.public.siteUrl.replace(/\/+$/, '')
const isPrivatePage = computed(() =>
  route.path.startsWith('/admin')
  || ['/login', '/register', '/settings', '/collections'].includes(route.path),
)
const canonicalUrl = computed(() => `${siteOrigin}${route.path === '/' ? '/' : route.path}`)

useHead(() => ({
  title: 'luohao-blog',
  titleTemplate: (title) => (title === 'luohao-blog' ? title : `${title} · luohao-blog`),
  htmlAttrs: { lang: 'zh-CN' },
  script: [{ innerHTML: themeScript }],
  link: isPrivatePage.value
    ? []
    : [{ rel: 'canonical', href: canonicalUrl.value }],
  meta: isPrivatePage.value
    ? [{ name: 'robots', content: 'noindex, nofollow' }]
    : [{ property: 'og:url', content: canonicalUrl.value }],
}))
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>
