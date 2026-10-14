<script setup lang="ts">
// The inline script applies the persisted theme class before first paint so
// SSR pages never flash the wrong color scheme.
const themeScript = `(function(){try{var p=localStorage.getItem('theme')||'system';var d=p==='dark'||(p==='system'&&window.matchMedia('(prefers-color-scheme: dark)').matches);document.documentElement.classList.toggle('dark',d);document.documentElement.style.colorScheme=d?'dark':'light';}catch(e){}})();`

const route = useRoute()
const config = useRuntimeConfig()
const publicOrigin = siteOrigin(config.public.siteUrl)
const isPrivatePage = computed(() =>
  route.path.startsWith('/admin')
  || ['/login', '/register', '/settings', '/collections', '/forgot-password', '/reset-password', '/verify-email'].some(path => route.path === path || route.path.startsWith(`${path}/`)),
)
const canonicalUrl = computed(() => siteURL(publicOrigin, route.path === '/' ? '/' : route.path))
const defaultImage = siteURL(publicOrigin, DEFAULT_SEO_IMAGE)

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
    : [],
}))

useSeoMeta({
  title: 'luohao-blog',
  description: DEFAULT_SEO_DESCRIPTION,
  ogTitle: 'luohao-blog',
  ogDescription: DEFAULT_SEO_DESCRIPTION,
  ogType: 'website',
  ogUrl: () => isPrivatePage.value ? undefined : canonicalUrl.value,
  ogImage: defaultImage,
  twitterCard: 'summary_large_image',
  twitterTitle: 'luohao-blog',
  twitterDescription: DEFAULT_SEO_DESCRIPTION,
  twitterImage: defaultImage,
})
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>
