<script setup lang="ts">
// Rich-text Markdown editor (Milkdown Crepe, the decision recorded in the M2
// plan). The editor library touches browser APIs, so it is imported lazily
// inside onMounted; the package CSS is style-only and safe to import at the
// top level. Emits the updated Markdown source on every change.
import type { Crepe } from '@milkdown/crepe'
import { editorViewCtx, parserCtx } from '@milkdown/kit/core'

import '@milkdown/crepe/theme/common/style.css'
import '@milkdown/crepe/theme/classic.css'

const props = defineProps<{ modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const rootEl = ref<HTMLDivElement | null>(null)
let crepe: Crepe | null = null

function insertMarkdown(markdown: string): boolean {
  if (!crepe)
    return false
  const nextMarkdown = `${crepe.getMarkdown().trimEnd()}\n\n${markdown}\n`
  crepe.editor.action((ctx) => {
    const view = ctx.get(editorViewCtx)
    const doc = ctx.get(parserCtx)(nextMarkdown)
    view.dispatch(view.state.tr.replaceWith(0, view.state.doc.content.size, doc.content).scrollIntoView())
    view.focus()
  })
  emit('update:modelValue', nextMarkdown)
  return true
}

defineExpose({ insertMarkdown })

onMounted(async () => {
  const { Crepe: CrepeClass } = await import('@milkdown/crepe')
  crepe = new CrepeClass({
    root: rootEl.value,
    defaultValue: props.modelValue,
  })
  crepe.on((listener) => {
    listener.markdownUpdated((_ctx, markdown, prevMarkdown) => {
      if (markdown !== prevMarkdown)
        emit('update:modelValue', markdown)
    })
  })
  await crepe.create()
})

onBeforeUnmount(async () => {
  await crepe?.destroy()
  crepe = null
})
</script>

<template>
  <div ref="rootEl" class="overflow-hidden rounded-xl border border-slate-300 bg-white dark:border-slate-600" />
</template>

<style>
/* The editor frame theme is light-only for now; keep it legible on dark
   backgrounds instead of shipping a second theme bundle. */
.dark .milkdown {
  color-scheme: light;
}
</style>
