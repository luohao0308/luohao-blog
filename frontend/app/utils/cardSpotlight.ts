// Pointer lighting follows the reference bento grid. Card geometry is read as a
// batch before writing CSS variables; links and native input are never captured.
export function mountCardSpotlight(root: HTMLElement) {
  const doc = root.ownerDocument
  const view = doc.defaultView
  if (!view || typeof view.matchMedia !== 'function') return { reset() {}, destroy() {} }
  const fine = view.matchMedia('(hover: hover) and (pointer: fine)')
  const reduce = view.matchMedia('(prefers-reduced-motion: reduce)')
  const touched = new Set<HTMLElement>()
  let frame: number | undefined
  let x = 0, y = 0, disposed = false
  function reset() {
    if (frame !== undefined) { view!.cancelAnimationFrame(frame); frame = undefined }
    root.removeAttribute('data-card-spotlight')
    for (const card of touched) {
      card.style.removeProperty('--card-x')
      card.style.removeProperty('--card-y')
    }
    touched.clear()
  }
  function paint() {
    frame = undefined
    if (disposed || !fine.matches || reduce.matches || doc.hidden) { reset(); return }
    const cards = [...root.querySelectorAll<HTMLElement>('.bento-card, .ui-panel')]
    const geometry = cards.map(card => ({ card, box: card.getBoundingClientRect() }))
    for (const old of touched) if (!old.isConnected) {
      old.style.removeProperty('--card-x'); old.style.removeProperty('--card-y'); touched.delete(old)
    }
    for (const { card, box } of geometry) {
      card.style.setProperty('--card-x', `${x - box.left}px`)
      card.style.setProperty('--card-y', `${y - box.top}px`)
      touched.add(card)
    }
    if (cards.length) root.dataset.cardSpotlight = 'true'
    else root.removeAttribute('data-card-spotlight')
  }
  function move(event: PointerEvent) {
    if (event.pointerType !== 'mouse' || !fine.matches || reduce.matches || doc.hidden) { reset(); return }
    x = event.clientX; y = event.clientY
    if (frame === undefined) frame = view!.requestAnimationFrame(paint)
  }
  root.addEventListener('pointermove', move)
  root.addEventListener('pointerleave', reset)
  root.addEventListener('pointercancel', reset)
  view.addEventListener('scroll', reset, { capture: true, passive: true })
  view.addEventListener('resize', reset)
  doc.addEventListener('visibilitychange', reset)
  fine.addEventListener('change', reset)
  reduce.addEventListener('change', reset)
  return {
    reset,
    destroy() {
      disposed = true
      reset()
      root.removeEventListener('pointermove', move)
      root.removeEventListener('pointerleave', reset)
      root.removeEventListener('pointercancel', reset)
      view.removeEventListener('scroll', reset, true)
      view.removeEventListener('resize', reset)
      doc.removeEventListener('visibilitychange', reset)
      fine.removeEventListener('change', reset)
      reduce.removeEventListener('change', reset)
    },
  }
}
