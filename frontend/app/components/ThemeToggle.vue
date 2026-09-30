<script setup lang="ts">
// Toggle cycles light -> dark (system preference is honored on first visit;
// after any manual toggle the explicit choice wins).
const { preference, toggle } = useTheme()

const isDark = computed(() => {
  if (preference.value === 'dark') return true
  if (preference.value === 'light') return false
  return false // 'system' resolves on init; icon shows sun while ambiguous
})
</script>

<template>
  <button
    type="button"
    class="rounded-lg p-2 text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100"
    :aria-label="isDark ? '切换到亮色模式' : '切换到暗色模式'"
    @click="toggle()"
  >
    <!-- sun (shown in dark mode: click to go light) -->
    <svg v-if="isDark" class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <circle cx="12" cy="12" r="4" />
      <path stroke-linecap="round" d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4l1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4m11.4-11.4l1.4-1.4" />
    </svg>
    <!-- moon -->
    <svg v-else class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
    </svg>
  </button>
</template>
