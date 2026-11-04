<script setup lang="ts">
defineProps<{ src: string, alt: string }>()
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLDialogElement | null>(null)
const failed = ref(false)
const fallbackClose = ref<HTMLButtonElement | null>(null)
async function showFallback() {
  failed.value = true
  await nextTick()
  fallbackClose.value?.focus()
}
let previousFocus: HTMLElement | null = null
let previousOverflow = ''
onMounted(() => {
  previousFocus = document.activeElement as HTMLElement | null
  previousOverflow = document.body.style.overflow
  if (!dialog.value?.showModal) { showFallback(); return }
  try {
    dialog.value.showModal()
    document.body.style.overflow = 'hidden'
  }
  catch { showFallback() }
})
onBeforeUnmount(() => {
  dialog.value?.close?.()
  document.body.style.overflow = previousOverflow
  if (previousFocus?.isConnected) previousFocus.focus({ preventScroll: true })
})
</script>

<template>
  <Teleport to="body">
    <section v-if="failed" aria-label="图片预览" class="image-fallback ui-panel p-4" @keydown.esc="emit('close')">
      <div class="flex justify-end pb-3">
        <button ref="fallbackClose" type="button" class="ui-button" @click="emit('close')">关闭预览</button>
      </div>
      <img :src="src" :alt="alt" class="preview-image">
    </section>
    <dialog v-else ref="dialog" aria-label="图片预览" class="image-dialog" @cancel.prevent="emit('close')" @close="emit('close')" @click="event => { if (event.target === dialog) emit('close') }">
      <div class="flex items-center justify-end p-3">
        <button type="button" class="ui-button" autofocus @click="emit('close')">关闭预览</button>
      </div>
      <img :src="src" :alt="alt" class="preview-image">
      <p v-if="alt" class="p-3 text-sm">{{ alt }}</p>
    </dialog>
  </Teleport>
</template>

<style scoped>
.image-fallback { position: fixed; bottom: 1rem; left: 50%; transform: translateX(-50%); width: min(92vw, 30rem); z-index: 100; box-shadow: 0 8px 32px rgb(0 0 0 / 20%); }
.image-dialog { position: fixed; inset: 0; margin: auto; width: min(94vw, 80rem); max-height: 94dvh; overflow: auto; border: 1px solid var(--border-default); border-radius: var(--radius-panel); background: var(--surface-overlay); color: var(--text-primary); padding: 0; }
.image-dialog::backdrop { background: rgb(0 0 0 / 75%); }
.preview-image { display: block; margin: auto; width: auto; max-width: 100%; height: auto; max-height: 70dvh; object-fit: contain; }
</style>
