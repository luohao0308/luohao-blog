<script setup lang="ts">
import type { ChatRequestStatus } from '~/composables/useChatArticle'
import { chatPlacement } from '~/utils/chatPlacement'

const props = defineProps<{ mascotSrc?: string }>()
const route = useRoute()
const article = useChatArticle()
const dialog = ref<HTMLDialogElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const opened = ref(false)
const expanded = ref(false)
const { user } = useAuth()
const userAvatar = computed(() => user.value?.avatar_url ? assetUrl(user.value.avatar_url) : '')
const userInitial = computed(() => Array.from(user.value?.display_name?.trim() ?? '')[0] ?? '访')
const failedAvatar = ref('')
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
  if (!expanded.value) {
    dialogPosition.value = { ...placement.style, width: `${width}px` }
    return
  }
  // Keep the edge nearest the mascot fixed as the portrait panel grows.
  const availableHeight = Number.parseFloat(placement.style.maxHeight!)
  const expandedWidth = Math.max(width, Math.min(22 * unit, window.innerWidth - 2 * unit, availableHeight * 9 / 16))
  dialogPosition.value = {
    ...placement.style,
    left: `${Math.max(unit, left + width - expandedWidth)}px`,
    width: `${expandedWidth}px`,
    height: `${Math.min(availableHeight, expandedWidth * 16 / 9)}px`,
  }
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
  expanded.value = false
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
watch([dragPosition, variant, expanded], updateClearance, { flush: 'post' })
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
  <dialog :id="`${titleId}-dialog`" ref="dialog" :aria-labelledby="titleId" class="floating-chat-dialog" :data-variant="variant" :class="{ 'is-expanded': expanded }" :style="dialogPosition" @close="onClosed" @compositionstart.capture="imeActive = true" @compositionend.capture="imeActive = false">
    <div class="floating-chat-heading">
      <div class="xiaowu-identity">
        <span v-if="variant === 'wechat'" class="xiaowu-avatar" aria-hidden="true">
          <PetSprite v-if="opened && props.mascotSrc" :src="props.mascotSrc" :size="26" />
          <span v-else>乌</span>
        </span>
        <h2 :id="titleId">小乌</h2>
      </div>
      <div class="floating-chat-actions">
        <button type="button" class="floating-chat-control" :aria-label="expanded ? '缩小聊天窗口' : '放大聊天窗口'" :title="expanded ? '缩小' : '放大'" @click="expanded = !expanded">
          <svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path :d="expanded ? 'M4 9h5V4m11 11h-5v5M4 15h5v5M20 9h-5V4' : 'M9 4H4v5m11-5h5v5M4 15v5h5m11-5v5h-5'" /></svg>
        </button>
        <button type="button" class="floating-chat-control" aria-label="关闭聊天" @click="close()">
          <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="m6 6 12 12M18 6 6 18" /></svg>
        </button>
      </div>
    </div>
    <div class="floating-chat-content">
      <ChatWindow v-if="mountedChat" :key="chatKey" :article-slug="scope === 'article' ? currentArticle?.slug : undefined" :variant="variant" embedded @status="onRequestStatus">
        <template #composer-action>
          <button v-if="currentArticle" type="button" class="floating-chat-scope" aria-label="切换提问范围" :aria-pressed="scope === 'article'" :title="scope === 'article' ? '当前：聊本文。点击切换到全站' : '当前：聊全站。点击切换到本文'" @click="changeScope(scope === 'article' ? 'global' : 'article')">
            <svg aria-hidden="true" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
              <path v-if="scope === 'article'" d="M6 3h8l4 4v14H6z M14 3v5h4 M9 12h6m-6 4h6" />
              <template v-else>
                <circle cx="12" cy="12" r="9" />
                <ellipse cx="12" cy="12" rx="4" ry="9" />
                <path d="M3 12h18" />
              </template>
            </svg>
            <span>{{ scope === 'article' ? '本文' : '全站' }}</span>
            <svg aria-hidden="true" width="9" height="9" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"><path d="m3 4 3-2 3 2m-6 4 3 2 3-2" /></svg>
          </button>
        </template>
        <template #user-avatar>
          <span class="message-avatar user-avatar" aria-hidden="true">
            <img v-if="userAvatar && failedAvatar !== userAvatar" :src="userAvatar" alt="" @error="failedAvatar = userAvatar">
            <span v-else>{{ userInitial }}</span>
          </span>
        </template>
        <template #assistant-avatar>
          <span class="message-avatar xiaowu-message-avatar" aria-hidden="true">
            <span v-if="mascotReady && props.mascotSrc" class="xiaowu-portrait" :style="{ backgroundImage: `url(${JSON.stringify(props.mascotSrc)})` }" />
            <span v-else>乌</span>
          </span>
        </template>
      </ChatWindow>
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
.floating-chat-actions { display: flex; align-items: center; gap: 2px; }
.floating-chat-control { display: inline-grid; place-items: center; width: 36px; height: 36px; border-radius: .5rem; color: var(--text-muted); }
.floating-chat-control:hover { background: var(--surface-hover); color: var(--text-primary); }
.floating-chat-content { min-width: 0; padding: .25rem .75rem .75rem; }
.floating-chat-dialog.is-expanded[open] { display: flex; flex-direction: column; overflow: hidden; }
.floating-chat-dialog.is-expanded .floating-chat-heading { flex: none; }
.floating-chat-dialog.is-expanded .floating-chat-content { flex: 1; min-height: 0; overflow-y: auto; }
.floating-chat-dialog.is-expanded :deep(.assistant-chat) { height: 100%; }
.floating-chat-dialog.is-expanded :deep(.assistant-history) { flex: 1; min-height: 0; max-height: none; }
.floating-chat-dialog.is-expanded :deep(.chat-composer) { margin-top: auto; }
.floating-chat-scope { display: inline-flex; align-items: center; gap: 3px; flex: none; min-height: 26px; padding: 0 .375rem 0 .125rem; border-right: 1px solid var(--border-default); color: var(--text-muted); font-size: .6875rem; white-space: nowrap; }
.floating-chat-scope:hover, .floating-chat-scope[aria-pressed='true'] { color: var(--accent); }
.message-avatar { display: inline-grid; place-items: center; flex: none; width: 24px; height: 24px; overflow: hidden; border-radius: .375rem; font-size: .6875rem; }
.user-avatar { background: var(--surface-accent); color: var(--text-primary); }
.user-avatar img { width: 100%; height: 100%; object-fit: cover; }
.xiaowu-message-avatar { background: #f8e5a9; color: #634f26; }
.xiaowu-portrait { width: 20px; aspect-ratio: 192 / 208; background-size: 800% 900%; background-repeat: no-repeat; }
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
