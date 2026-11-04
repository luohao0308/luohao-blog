export interface ChatAnchorRect { left: number, right: number, top: number, bottom: number }

// Prefer the larger free area above/below the pet; short screens use the
// available viewport instead of squeezing the composer into a tiny strip.
export function chatPlacement(rect: ChatAnchorRect, width: number, height: number, unit = 16, maxWidth = 28) {
  const margin = unit
  const gap = unit * .75
  const panelWidth = Math.max(0, Math.min(maxWidth * unit, width - 2 * margin))
  const left = Math.max(margin, Math.min(rect.right - panelWidth, width - margin - panelWidth))
  const above = Math.max(0, rect.top - margin - gap)
  const below = Math.max(0, height - rect.bottom - margin - gap)
  const style: Record<string, string> = { left: `${left}px`, right: 'auto', top: 'auto', bottom: 'auto' }
  if (Math.max(above, below) < 15 * unit) {
    style.top = `${margin}px`
    style.maxHeight = `${Math.max(0, height - 2 * margin)}px`
  }
  else if (above >= below) {
    style.bottom = `${Math.max(margin, height - rect.top + gap)}px`
    style.maxHeight = `${above}px`
  }
  else {
    style.top = `${Math.max(margin, rect.bottom + gap)}px`
    style.maxHeight = `${below}px`
  }
  // Only reserve space for the right-hand contact button while the pet is
  // actually in that bottom corner; dragging upward must not hide contacts.
  const inCorner = rect.right > width - 8 * unit && rect.top > height - 15 * unit
  const clearance = Math.min(inCorner ? Math.max(5.5 * unit, height - rect.top + unit) : 5.5 * unit, Math.max(0, height - 4 * unit))
  return { style, clearance: `${clearance}px` }
}
