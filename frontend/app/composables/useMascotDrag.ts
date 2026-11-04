import type { CSSProperties, Ref } from 'vue'

export function useMascotDrag(trigger: Ref<HTMLButtonElement | null>, enabled: Ref<boolean>) {
  const position = ref<{ x: number, y: number } | null>(null)
  const dragging = ref(false)
  let pointer: { id: number, startX: number, startY: number, x: number, y: number, width: number, height: number, element: HTMLButtonElement } | null = null
  let pending: { x: number, y: number } | null = null
  let frame: number | null = null
  let blockClick = false
  let mounted = false
  const style = computed<CSSProperties>(() => enabled.value && position.value ? {
    left: '0', top: '0', right: 'auto', bottom: 'auto',
    transform: `translate3d(${position.value.x}px, ${position.value.y}px, 0)`,
  } : {})

  function bounded(x: number, y: number) {
    const rect = pointer ?? trigger.value?.getBoundingClientRect()
    return {
      x: Math.max(16, Math.min(x, Math.max(16, window.innerWidth - (rect?.width ?? 0) - 16))),
      y: Math.max(16, Math.min(y, Math.max(16, window.innerHeight - (rect?.height ?? 0) - 16))),
    }
  }
  function discardFrame() {
    if (frame !== null) cancelAnimationFrame(frame)
    frame = null
    pending = null
  }
  function flush() {
    if (frame !== null) cancelAnimationFrame(frame)
    frame = null
    if (pending) position.value = bounded(pending.x, pending.y)
    pending = null
  }
  function finish(cancelled: boolean) {
    const old = pointer
    pointer = null
    if (cancelled) discardFrame()
    else flush()
    blockClick ||= !!old && (cancelled || dragging.value)
    dragging.value = false
    if (old?.element.hasPointerCapture(old.id)) old.element.releasePointerCapture(old.id)
  }
  function clampToViewport() {
    if (pointer) finish(true)
    if (position.value) position.value = bounded(position.value.x, position.value.y)
  }
  function onPointerDown(event: PointerEvent) {
    if (!event.isPrimary || event.button !== 0 || pointer) return
    blockClick = false
    if (!enabled.value || !trigger.value) return
    const element = trigger.value
    const rect = element.getBoundingClientRect()
    pointer = { id: event.pointerId, startX: event.clientX, startY: event.clientY, x: position.value?.x ?? rect.left, y: position.value?.y ?? rect.top, width: rect.width, height: rect.height, element }
    try { element.setPointerCapture(event.pointerId) }
    catch { pointer = null }
  }
  function update(event: PointerEvent) {
    if (!pointer || pointer.id !== event.pointerId) return
    const dx = event.clientX - pointer.startX, dy = event.clientY - pointer.startY
    if (!dragging.value && Math.hypot(dx, dy) < 6) return
    dragging.value = true
    pending = bounded(pointer.x + dx, pointer.y + dy)
  }
  function onPointerMove(event: PointerEvent) {
    if (pointer?.id !== event.pointerId) return
    update(event)
    if (!pending) return
    event.preventDefault()
    if (frame === null) frame = requestAnimationFrame(flush)
  }
  function onPointerUp(event: PointerEvent) {
    if (pointer?.id !== event.pointerId) return
    update(event)
    finish(false)
  }
  function onPointerCancel(event: PointerEvent) {
    if (pointer?.id === event.pointerId) finish(true)
  }
  function onLostCapture(event: PointerEvent) { onPointerCancel(event) }
  function suppressClick(event: MouseEvent): boolean {
    if (!blockClick || event.detail === 0) return false
    event.preventDefault()
    event.stopImmediatePropagation()
    blockClick = false
    return true
  }
  function onKeydown(event: KeyboardEvent) {
    if (!enabled.value || event.altKey || event.ctrlKey || event.metaKey) return
    const delta: Record<string, [number, number]> = { ArrowLeft: [-16, 0], ArrowRight: [16, 0], ArrowUp: [0, -16], ArrowDown: [0, 16] }
    const step = delta[event.key]
    const rect = trigger.value?.getBoundingClientRect()
    if (!step || !rect) return
    event.preventDefault()
    if (pointer) finish(true)
    position.value = bounded((position.value?.x ?? rect.left) + step[0], (position.value?.y ?? rect.top) + step[1])
  }
  watch(enabled, value => {
    finish(true)
    if (!mounted) return
    window.removeEventListener('resize', clampToViewport)
    if (value) {
      clampToViewport()
      window.addEventListener('resize', clampToViewport)
    }
  })
  onMounted(() => {
    mounted = true
    if (enabled.value) window.addEventListener('resize', clampToViewport)
  })
  onBeforeUnmount(() => {
    mounted = false
    finish(true)
    window.removeEventListener('resize', clampToViewport)
  })
  return { position, dragging, style, onPointerDown, onPointerMove, onPointerUp, onPointerCancel, onLostCapture, suppressClick, onKeydown, clampToViewport }
}
