<script setup lang="ts">
import type { ChatRequestStatus } from '~/composables/useChatArticle'
import { chatPlacement } from '~/utils/chatPlacement'

const props = defineProps<{ mascotSrc?: string }>()
const route = useRoute()
const article = useChatArticle()
const dialog = ref<HTMLDialogElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const opened = ref(false)
const mountedChat = ref(false)
const openError = ref('')
const scope = ref<'article' | 'global'>('global')
const revision = ref(0)
const titleId = useId()
const mascotReady = ref(false)
const drag = useMascotDrag(trigger, mascotReady)
const { position: dragPosition, dragging, style: dragStyle } = drag
const compact = ref(false)
const pet = usePetActivity(mascotReady, opened, dragging, dragPosition)
const { state: petState, greet } = pet
let triggerObserver: ResizeObserver | undefined
let compactPreference: MediaQueryList | undefined
let previousClearance = ''
const variant = computed(() => route.query.preview === 'xiaowu-a' ? 'bubble' : 'wechat')
const tailPosition = ref<Record<string, string>>()
const dialogPosition = ref<Record<string, string>>({})

function resetPet() { pet.onRequestStatus('idle') }
function onRequestStatus(status: ChatRequestStatus) {
  pet.onRequestStatus(status === 'working' ? 'pending' : status)
}
function updateClearance() {
  if (!trigger.value) return
  const unit = Number.parseFloat(getComputedStyle(document.documentElement).fontSize) || 16
  const panelWidth = variant.value === 'bubble' ? 17 : 18
  const anchor = trigger.value.getBoundingClientRect()
  const placement = chatPlacement(anchor, window.innerWidth, window.innerHeight, unit, panelWidth)
  const left = Number.parseFloat(placement.style.left!)
  const width = Math.min(panelWidth * unit, window.innerWidth - 2 * unit)
  const center = Math.max(left + 18, Math.min((anchor.left + anchor.right) / 2, left + width - 18))
  tailPosition.value = placement.style.bottom !== 'auto'
    ? { left: `${center}px`, bottom: `calc(${placement.style.bottom} - 6px)` }
    : Number.parseFloat(placement.style.top!) > unit
      ? { left: `${center}px`, top: `calc(${placement.style.top} - 6px)` }
      : undefined
  document.documentElement.style.setProperty('--floating-chat-clearance', placement.clearance)
  dialogPosition.value = { ...placement.style, width: `min(${panelWidth}rem, calc(100vw - 2rem))` }
}
function onTriggerResize() { drag.clampToViewport(); updateClearance() }
function updateCompact() { compact.value = !!compactPreference?.matches }
const imeActive = ref(false)
let opening = 0
let disposed = false

const currentArticle = computed(() => {
  const value = article.value
  return value && route.path.replace(/\/$/, '') === `/posts/${encodeURIComponent(value.slug)}` ? value : null
})
const chatKey = computed(() => `${revision.value}:${scope.value}:${currentArticle.value?.slug ?? ''}`)

function restoreFocus() {
  if (!disposed && trigger.value?.isConnected) trigger.value.focus({ preventScroll: true })
}
function close(returnFocus = true) {
  opening++
  imeActive.value = false
  openError.value = ''
  opened.value = false
  if (dialog.value?.open) dialog.value.close()
  if (returnFocus) restoreFocus()
}
function onClosed() {
  if (dialog.value?.open) return
  opened.value = false
}
async function focusComposer() {
  await nextTick()
  if (opened.value && !disposed) dialog.value?.querySelector<HTMLTextAreaElement>('textarea')?.focus({ preventScroll: true })
}
async function open() {
  greet()
  const version = ++opening
  openError.value = ''
  mountedChat.value = true
  await nextTick()
  if (disposed || version !== opening || !dialog.value) return
  try {
    updateClearance()
    if (!dialog.value.open) dialog.value.show()
    opened.value = true
    await focusComposer()
  }
  catch (error) {
    if (disposed || version !== opening) return
    if (dialog.value?.open) dialog.value.close()
    opened.value = false
    openError.value = '问答窗口暂时无法打开，请重试。'
    console.error('[FloatingChat] Dialog open failed', error)
    restoreFocus()
  }
}
function onTriggerClick(event: MouseEvent) {
  if (drag.suppressClick(event)) return
  if (opened.value) close()
  else void open()
}
watch([dragPosition, variant], updateClearance, { flush: 'post' })
function changeScope(value: 'article' | 'global') {
  if (scope.value === value || (value === 'article' && !currentArticle.value)) return
  scope.value = value
  resetPet()
  // A new ChatWindow aborts its old request and owns a fresh question history.
  revision.value++
  void focusComposer()
}
function onOutsidePointer(event: PointerEvent) {
  if (!opened.value || !(event.target instanceof Node)) return
  if (dialog.value?.contains(event.target) || trigger.value?.contains(event.target)) return
  close(false)
}
function onAssistantKeydown(event: KeyboardEvent) {
  if (!opened.value || event.defaultPrevented || event.key !== 'Escape' || imeActive.value || event.isComposing || event.keyCode === 229) return
  event.preventDefault()
  event.stopPropagation()
  close()
}
watch(() => route.path, () => {
  close()
  mountedChat.value = false
  revision.value++
  scope.value = 'global'
  resetPet()
})
watch(() => currentArticle.value?.slug, () => {
  scope.value = 'global'
  revision.value++
})
watch(() => route.hash, () => { if (opened.value) close() })
onMounted(() => {
  scope.value = 'global'
  previousClearance = document.documentElement.style.getPropertyValue('--floating-chat-clearance')
  compactPreference = window.matchMedia('(max-width: 640px)')
  compactPreference.addEventListener('change', updateCompact)
  updateCompact()
  triggerObserver = new ResizeObserver(onTriggerResize)
  if (trigger.value) triggerObserver.observe(trigger.value)
  window.addEventListener('resize', updateClearance)
  document.addEventListener('pointerdown', onOutsidePointer, true)
  document.addEventListener('keydown', onAssistantKeydown)
  updateClearance()
})
onBeforeUnmount(() => {
  disposed = true
  opening++
  dialog.value?.close()
  resetPet()
  triggerObserver?.disconnect()
  compactPreference?.removeEventListener('change', updateCompact)
  window.removeEventListener('resize', updateClearance)
  document.removeEventListener('pointerdown', onOutsidePointer, true)
  document.removeEventListener('keydown', onAssistantKeydown)
  if (previousClearance) document.documentElement.style.setProperty('--floating-chat-clearance', previousClearance)
  else document.documentElement.style.removeProperty('--floating-chat-clearance')
})
</script>

<template>
  <Teleport to="body">
  <button ref="trigger" type="button" class="floating-chat-trigger" :class="{ 'ui-button': !props.mascotSrc, 'mascot-slot': props.mascotSrc, 'has-mascot': mascotReady, 'is-dragging': dragging }" :style="dragStyle" aria-label="打开小乌" aria-haspopup="dialog" :aria-expanded="opened" :aria-controls="`${titleId}-dialog`" :aria-describedby="mascotReady ? `${titleId}-drag-help` : undefined" draggable="false" @pointerenter="greet" @focus="greet" @pointerdown="drag.onPointerDown" @pointermove="drag.onPointerMove" @pointerup="drag.onPointerUp" @pointercancel="drag.onPointerCancel" @lostpointercapture="drag.onLostCapture" @keydown="drag.onKeydown" @click="onTriggerClick">
    <PetSprite v-if="props.mascotSrc" v-show="mascotReady" :src="props.mascotSrc" :state="petState" :size="compact ? 64 : 80" @ready="mascotReady = $event" />
    <svg v-if="!mascotReady" aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"><path d="M20 11.5a8 8 0 0 1-8 8H5l-4 3v-11a8 8 0 0 1 8-8h3a8 8 0 0 1 8 8Z" stroke-linejoin="round" /><path d="M7 11h8M7 15h5" stroke-linecap="round" /></svg>
    <span v-if="!props.mascotSrc">小乌</span>
  </button>
  <span v-if="mascotReady" :id="`${titleId}-drag-help`" class="sr-only">拖动或用方向键移动，按回车打开问答。</span>
  <p v-if="openError" role="alert" class="floating-chat-error">{{ openError }}</p>
  <span v-if="opened && variant === 'bubble' && tailPosition" class="xiaowu-tail" :style="tailPosition" aria-hidden="true" />
  <dialog :id="`${titleId}-dialog`" ref="dialog" :aria-labelledby="titleId" class="floating-chat-dialog" :data-variant="variant" :style="dialogPosition" @close="onClosed" @compositionstart.capture="imeActive = true" @compositionend.capture="imeActive = false">
    <div class="floating-chat-heading">
      <div class="xiaowu-identity">
        <span v-if="variant === 'wechat'" class="xiaowu-avatar" aria-hidden="true">
          <PetSprite v-if="opened && props.mascotSrc" :src="props.mascotSrc" :size="26" />
          <span v-else>乌</span>
        </span>
        <h2 :id="titleId">小乌</h2>
      </div>
      <button type="button" class="floating-chat-close" aria-label="关闭聊天" @click="close()">
        <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="m6 6 12 12M18 6 6 18" /></svg>
      </button>
    </div>
    <div class="floating-chat-content">
      <div v-if="currentArticle" class="floating-chat-context">
        <p class="floating-chat-article">{{ currentArticle.title }}</p>
        <div class="floating-chat-scopes" role="group" aria-label="提问范围">
          <button type="button" :aria-pressed="scope === 'article'" @click="changeScope('article')">聊本文</button>
          <button type="button" :aria-pressed="scope === 'global'" @click="changeScope('global')">聊全站</button>
        </div>
      </div>
      <ChatWindow v-if="mountedChat" :key="chatKey" :article-slug="scope === 'article' ? currentArticle?.slug : undefined" :variant="variant" embedded @status="onRequestStatus" />
    </div>
  </dialog>
  </Teleport>
</template>

<style scoped>
.floating-chat-trigger { position: fixed; z-index: 45; right: max(1.5rem, env(safe-area-inset-right)); bottom: max(1.5rem, env(safe-area-inset-bottom)); gap: .5rem; min-height: 44px; border-radius: 999px; box-shadow: 0 6px 22px rgb(17 33 60 / 20%); font-size: .875rem; font-weight: 600; }
.floating-chat-trigger.mascot-slot { display: inline-grid; place-items: center; width: 80px; aspect-ratio: 192 / 208; padding: 0; border-radius: .75rem; background: transparent; color: var(--text-muted); box-shadow: none; }
.mascot-slot:not(.has-mascot) > svg { box-sizing: content-box; padding: 8px; border: 1px solid var(--border-default); border-radius: 50%; background: var(--surface-overlay); }
.floating-chat-trigger.has-mascot { padding: 0; border-radius: .75rem; background: transparent; color: var(--text-primary); box-shadow: none; touch-action: none; user-select: none; -webkit-user-select: none; cursor: grab; }
.floating-chat-trigger.is-dragging { cursor: grabbing; }
.floating-chat-trigger.has-mascot :deep(canvas) { filter: drop-shadow(0 3px 3px rgb(0 0 0 / 10%)); }
.floating-chat-error { position: fixed; z-index: 45; right: 1rem; bottom: max(5rem, calc(env(safe-area-inset-bottom) + 4rem)); max-width: min(20rem, calc(100vw - 2rem)); padding: .75rem 1rem; border: 1px solid var(--border-default); border-radius: .75rem; background: var(--surface-overlay); color: var(--status-error); font-size: .875rem; }
.floating-chat-dialog { position: fixed; z-index: 46; max-width: calc(100vw - 2rem); margin: 0; padding: 0; overflow: auto; overscroll-behavior: contain; border: 1px solid var(--border-default); border-radius: .875rem; background: var(--surface-overlay); color: var(--text-primary); box-shadow: 0 10px 32px rgb(17 33 60 / 16%); }
.floating-chat-heading { position: sticky; top: 0; z-index: 1; display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: .5rem .5rem .25rem 1rem; background: var(--surface-overlay); }
.floating-chat-heading h2 { font-size: .875rem; font-weight: 500; line-height: 1.5; }
.floating-chat-close { display: inline-flex; align-items: center; justify-content: center; width: 44px; height: 44px; border-radius: .5rem; color: var(--text-muted); }
.floating-chat-close:hover { background: var(--surface-hover); color: var(--text-primary); }
.floating-chat-content { min-width: 0; padding: .25rem .75rem .75rem; }
.floating-chat-context { display: flex; flex-direction: column; align-items: flex-start; gap: .625rem; margin-bottom: .875rem; }
.floating-chat-article { overflow-wrap: anywhere; color: var(--text-muted); font-size: .8125rem; line-height: 1.7; }
.floating-chat-scopes { display: inline-flex; gap: .125rem; padding: .25rem; border-radius: .625rem; background: var(--surface-page); }
.floating-chat-scopes button { min-height: 36px; padding: .375rem .875rem; border-radius: .4375rem; background: transparent; color: var(--text-muted); font-size: .75rem; }
.floating-chat-scopes button[aria-pressed='true'] { background: var(--surface-overlay); color: var(--text-primary); box-shadow: 0 1px 4px rgb(17 33 60 / 8%); }
@media (max-width: 640px) { .floating-chat-trigger { right: max(1rem, env(safe-area-inset-right)); } .floating-chat-trigger.mascot-slot { width: 64px; } }
.xiaowu-identity { display: flex; align-items: center; gap: .5rem; }
.xiaowu-avatar { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 8px; background: #f8e5a9; color: #634f26; font-size: .75rem; }
.floating-chat-dialog[data-variant='bubble'], .xiaowu-tail { --bubble-paper: #fffaf0; --bubble-edge: #e9dfcd; background: var(--bubble-paper); border-color: var(--bubble-edge); }
:global(.dark) .floating-chat-dialog[data-variant='bubble'], :global(.dark) .xiaowu-tail { --bubble-paper: #29251f; --bubble-edge: #5a4e3b; }
.floating-chat-dialog[data-variant='bubble'] .floating-chat-heading { background: var(--bubble-paper); }
.floating-chat-dialog[data-variant='bubble'] { border-radius: 1.125rem; }
.xiaowu-tail { position: fixed; z-index: 46; width: 12px; height: 12px; border: 1px solid var(--bubble-edge); transform: translateX(-50%) rotate(45deg); pointer-events: none; }
.floating-chat-dialog[data-variant='wechat'] .floating-chat-heading { border-bottom: 1px solid var(--border-default); padding-block: .25rem; }
.floating-chat-dialog[data-variant='wechat'] .floating-chat-content { padding: 0; }
</style>
