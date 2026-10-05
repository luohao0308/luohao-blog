<script setup lang="ts">
// Compact shared page header: one title line plus an optional one-line
// description. The eyebrow slot sits above the title (e.g. a back link);
// legacy pages used a tall breadcrumb + title + lede block instead.
defineProps<{
  title: string
  description?: string
}>()
</script>

<template>
  <header class="space-y-1.5">
    <slot name="eyebrow" />
    <h1 class="break-words text-2xl font-bold tracking-tight">
      {{ title }}
    </h1>
    <p v-if="description" class="text-sm leading-6 text-slate-500 dark:text-slate-400">
      {{ description }}
    </p>
  </header>
</template>
