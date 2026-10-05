<script setup lang="ts">
// 文章 dropdown: the content-taxonomy pages (全部文章/分类/标签/归档) collapse
// under one nav entry. Opens on hover and on keyboard focus (focus-within);
// no JS state needed.
const links = [
  { to: '/posts', label: '全部文章' },
  { to: '/categories', label: '分类' },
  { to: '/tags', label: '标签' },
  { to: '/archives', label: '归档' },
]
</script>

<template>
  <div class="group relative">
    <button
      type="button"
      class="flex items-center gap-1 text-slate-600 transition-colors group-hover:text-slate-900 dark:text-slate-400 dark:group-hover:text-slate-100"
    >
      文章
      <svg
        class="h-3.5 w-3.5 transition-transform group-hover:rotate-180"
        viewBox="0 0 20 20"
        fill="currentColor"
        aria-hidden="true"
      >
        <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 0 1 1.06.02L10 11.17l3.71-3.94a.75.75 0 1 1 1.08 1.04l-4.25 4.5a.75.75 0 0 1-1.08 0l-4.25-4.5a.75.75 0 0 1 .02-1.06Z" clip-rule="evenodd" />
      </svg>
    </button>
    <div
      class="invisible absolute left-1/2 top-full z-50 -translate-x-1/2 pt-2 opacity-0 transition-all duration-150 group-focus-within:visible group-focus-within:opacity-100 group-hover:visible group-hover:opacity-100"
    >
      <div
        class="w-40 rounded-lg border border-slate-200 bg-white py-1 shadow-lg dark:border-slate-700 dark:bg-slate-900"
        role="menu"
      >
        <NuxtLink
          v-for="link in links"
          :key="link.to"
          :to="link.to"
          role="menuitem"
          class="block px-4 py-2 text-sm text-slate-600 transition-colors hover:bg-slate-50 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-white"
        >
          {{ link.label }}
        </NuxtLink>
      </div>
    </div>
  </div>
</template>
