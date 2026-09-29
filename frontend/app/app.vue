<script setup lang="ts">
useHead({
  title: 'luohao-blog',
  titleTemplate: (title) => (title === 'luohao-blog' ? title : `${title} · luohao-blog`),
})
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>
