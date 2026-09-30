<script setup lang="ts">
// The inline script applies the persisted theme class before first paint so
// SSR pages never flash the wrong color scheme.
const themeScript = `(function(){try{var p=localStorage.getItem('theme')||'system';var d=p==='dark'||(p==='system'&&window.matchMedia('(prefers-color-scheme: dark)').matches);document.documentElement.classList.toggle('dark',d);document.documentElement.style.colorScheme=d?'dark':'light';}catch(e){}})();`

useHead({
  title: 'luohao-blog',
  titleTemplate: (title) => (title === 'luohao-blog' ? title : `${title} · luohao-blog`),
  htmlAttrs: { lang: 'zh-CN' },
  script: [{ innerHTML: themeScript }],
})
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>
