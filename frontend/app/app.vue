<script setup lang="ts">
import { HOME_ENTRANCE_BOOT_SCRIPT } from '~/utils/homeEntranceBoot'
// The inline script applies the persisted theme class before first paint so
// SSR pages never flash the wrong color scheme.
const themeScript = `(function(){var p='light';try{var s=localStorage.getItem('theme');if(s==='dark'||s==='system')p=s;}catch(e){}var d=p==='dark'||(p==='system'&&window.matchMedia('(prefers-color-scheme: dark)').matches);document.documentElement.classList.toggle('dark',d);document.documentElement.style.colorScheme=d?'dark':'light';})();`

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
  script: [{ innerHTML: themeScript }, { innerHTML: HOME_ENTRANCE_BOOT_SCRIPT }],
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
