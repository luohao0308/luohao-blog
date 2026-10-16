<script setup lang="ts">
// Rich-text Markdown editor (Milkdown Crepe, the decision recorded in the M2
// plan). The editor library touches browser APIs, so it is imported lazily
// inside onMounted; the package CSS is style-only and safe to import at the
// top level. Emits the updated Markdown source on every change.
import type { Crepe } from '@milkdown/crepe'
import { editorViewCtx, parserCtx, serializerCtx } from '@milkdown/kit/core'

import { Plugin } from '@milkdown/kit/prose/state'
import { $prose, $remark } from '@milkdown/kit/utils'

import '@milkdown/crepe/theme/common/style.css'
import '@milkdown/crepe/theme/classic.css'

const props = defineProps<{ modelValue: string, disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const rootEl = ref<HTMLDivElement | null>(null)
let crepe: Crepe | null = null

function insertMarkdown(markdown: string): boolean {
  if (!crepe || props.disabled)
    return false
  const nextMarkdown = `${crepe.getMarkdown().trimEnd()}\n\n${markdown}\n`
  crepe.editor.action((ctx) => {
    const view = ctx.get(editorViewCtx)
    const doc = ctx.get(parserCtx)(nextMarkdown)
    view.dispatch(view.state.tr.replaceWith(0, view.state.doc.content.size, doc.content).scrollIntoView())
    view.focus()
  })
  return true
}

defineExpose({ insertMarkdown })

const state = ref<'loading' | 'ready' | 'error'>('loading')
let disposed = false
let syncingSource = false

function replaceMarkdown(markdown: string) {
  syncingSource = true
  try {
    crepe?.editor.action((ctx) => {
      const view = ctx.get(editorViewCtx)
      const doc = ctx.get(parserCtx)(markdown)
      view.dispatch(view.state.tr.replaceWith(0, view.state.doc.content.size, doc.content))
    })
  }
  finally {
    syncingSource = false
  }
}

async function initialize() {
  if (state.value === 'ready' || disposed)
    return
  state.value = 'loading'
  let candidate: Crepe | null = null
  try {
    const { Crepe: CrepeClass } = await import('@milkdown/crepe')
    if (disposed)
      return
    candidate = new CrepeClass({ root: rootEl.value, defaultValue: props.modelValue })
    // Remark represents an omitted image title as null. Crepe 7.22's image
    // schemas require strings and otherwise swallow/drop that image while
    // parsing. Normalize at the AST boundary for initial content and updates.
    candidate.editor.use($remark('image-title-default', () => () => (tree) => {
      function normalize(node: { type: string, title?: unknown, children?: unknown[] }) {
        if ((node.type === 'image' || node.type === 'image-block') && node.title == null)
          node.title = ''
        node.children?.forEach(child => normalize(child as Parameters<typeof normalize>[0]))
      }
      normalize(tree)
    }))
    // Milkdown's markdownUpdated listener debounces for 200ms and cancels on
    // destroy. Observe actual transactions synchronously so save/leave cannot
    // miss a final keystroke, including edits immediately before blur.
    candidate.editor.use($prose(ctx => new Plugin({
      view: () => ({
        update(view, previous) {
          if (state.value === 'ready' && !syncingSource && !view.state.doc.eq(previous.doc))
            emit('update:modelValue', ctx.get(serializerCtx)(view.state.doc))
        },
      }),
    })))
    await candidate.create()
    if (disposed) {
      await candidate.destroy()
      return
    }
    crepe = candidate
    // Inputs and image insertion may change the source during initialization.
    replaceMarkdown(props.modelValue)
    crepe.setReadonly(!!props.disabled)
    state.value = 'ready'
  }
  catch {
    await candidate?.destroy().catch(() => {})
    if (!disposed) {
      crepe = null
      rootEl.value?.replaceChildren()
      state.value = 'error'
    }
  }
}

watch(() => props.disabled, value => crepe?.setReadonly(!!value))
watch(() => props.modelValue, (value) => {
  if (state.value === 'ready' && crepe?.getMarkdown() !== value)
    replaceMarkdown(value)
})
onMounted(initialize)
onBeforeUnmount(() => {
  disposed = true
  void crepe?.destroy().catch(() => {})
  crepe = null
})
</script>

<template>
  <div class="editor-frame overflow-hidden rounded-xl border border-slate-300 dark:border-slate-600" :aria-busy="state === 'loading'">
    <p v-if="state === 'loading'" role="status" class="p-3 text-sm">正在加载编辑器，可先编辑下方 Markdown…</p>
    <div v-if="state === 'error'" role="alert" class="space-y-2 p-3 text-sm">
      <p>编辑器加载失败，正文已保留，可继续编辑 Markdown 并保存。</p>
      <button type="button" class="ui-button" :disabled="disabled" @click="initialize">重试加载编辑器</button>
      <p>若重试仍失败，浏览器可能缓存了加载失败结果；请先保存正文，再刷新页面。</p>
    </div>
    <textarea
      v-if="state !== 'ready'"
      aria-label="Markdown 正文"
      :value="modelValue"
      :disabled="disabled"
      rows="12"
      class="w-full p-4"
      @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
    />
    <div v-show="state === 'ready'" ref="rootEl" />
  </div>
</template>

<style>
.editor-frame {
  background: var(--surface-input);
  color: var(--text-primary);
}
.dark .editor-frame .milkdown {
  color-scheme: dark;
  --crepe-color-background: var(--surface-input);
  --crepe-color-on-background: var(--text-primary);
  --crepe-color-surface: var(--surface-panel);
  --crepe-color-surface-low: var(--surface-page);
  --crepe-color-on-surface: var(--text-primary);
  --crepe-color-on-surface-variant: var(--text-muted);
  --crepe-color-outline: var(--border-default);
  --crepe-color-primary: var(--accent);
  --crepe-color-secondary: #334155;
  --crepe-color-on-secondary: #e2e8f0;
  --crepe-color-inverse: #e2e8f0;
  --crepe-color-on-inverse: #0f172a;
  --crepe-color-inline-code: #fda4af;
  --crepe-color-error: #fca5a5;
  --crepe-color-hover: #1e293b;
  --crepe-color-selected: #334155;
  --crepe-color-inline-area: #1e293b;
}
</style>
