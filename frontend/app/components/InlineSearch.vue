<script setup lang="ts">
const props = defineProps<{ label: string }>()
const query = defineModel<string>({ default: '' })
const open = ref(false)
const keyboardFocus = ref(false)
const input = ref<HTMLInputElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const searchId = useId()
async function toggle(event: MouseEvent) {
  keyboardFocus.value = event.detail === 0
  open.value = !open.value
  await nextTick()
  if (open.value) input.value?.focus()
  else trigger.value?.focus()
}
function close() {
  open.value = false
  trigger.value?.focus()
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex min-w-0 items-center gap-3">
      <slot name="heading" />
      <div class="flex min-w-0 flex-1 items-center" @keydown.tab="keyboardFocus = true" @pointerdown="keyboardFocus = false">
        <Transition name="inline-search">
          <div v-if="open" :id="searchId" class="inline-search-field flex min-w-0 items-center gap-1" @keydown.esc.prevent="close">
            <input ref="input" v-model="query" type="text" role="searchbox" inputmode="search" enterkeyhint="search" :aria-label="props.label" :placeholder="props.label" class="inline-search-input ui-field w-full min-w-0 text-sm" :class="{ 'keyboard-focus': keyboardFocus }">
          </div>
        </Transition>
        <button ref="trigger" type="button" :aria-label="props.label" :title="props.label" :aria-expanded="open" :aria-controls="searchId" class="flex size-10 shrink-0 items-center justify-center rounded-full text-slate-500 transition-colors hover:bg-slate-100 hover:text-blue-600 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-blue-300" @click="toggle">
          <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" class="size-5">
            <circle cx="10.5" cy="10.5" r="6.5" />
            <path d="m16 16 4.5 4.5" stroke-linecap="round" />
          </svg>
        </button>
        <div v-if="$slots.actions" class="ml-1 flex shrink-0 items-center">
          <slot name="actions" />
        </div>
      </div>
    </div>
    <Transition name="search-options">
      <div v-if="open && $slots.filters" class="search-options" @keydown.esc.prevent="close"><slot name="filters" /></div>
    </Transition>
  </div>
</template>

<style scoped>
.inline-search-field { width: 23rem; max-width: calc(100% - 2.5rem); }
.inline-search-input:focus {
  border-color: var(--border-default);
  box-shadow: none;
  outline: none;
}
.inline-search-input.keyboard-focus:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 3px;
}
.inline-search-input { appearance: none; }
.inline-search-enter-active, .inline-search-leave-active { overflow: hidden; transition: width 220ms ease, opacity 180ms ease; }
.inline-search-enter-from, .inline-search-leave-to { width: 0; opacity: 0; }
.search-options-enter-active, .search-options-leave-active { transition: opacity 180ms ease, transform 180ms ease; }
.search-options-enter-from, .search-options-leave-to { opacity: 0; transform: translateY(-4px); }
@media (prefers-reduced-motion: reduce) { .inline-search-enter-active, .inline-search-leave-active, .search-options-enter-active, .search-options-leave-active { transition: none; } }
</style>
