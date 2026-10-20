interface JourneyScrollGeometry { scroll: number, start: number, distance: number, count: number }

/** Let an interrupted camera journey finish at a readable chapter after scroll input rests. */
export function createJourneyScrollSnap(read: () => JourneyScrollGeometry | null, move: (top: number) => void) {
  let previousScroll: number | undefined
  let direction = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  const cancel = () => { if (timer !== undefined) clearTimeout(timer); timer = undefined }
  const schedule = () => {
    cancel()
    const observed = read()
    if (!observed) { previousScroll = undefined; return }
    const delta = observed.scroll - (previousScroll ?? observed.start)
    if (Math.abs(delta) > .5) direction = Math.sign(delta)
    previousScroll = observed.scroll
    timer = setTimeout(() => {
      timer = undefined
      const geometry = read()
      if (!geometry || geometry.distance <= 0 || geometry.count < 2) return
      const { scroll, start, distance, count } = geometry
      const progress = (scroll - start) / distance
      if (progress < 0 || progress > 1) return
      const position = progress * (count - 1)
      const nearestTop = start + distance * Math.round(position) / (count - 1)
      if (Math.abs(nearestTop - scroll) <= 2) return
      // Complete this gesture in its direction; small wheel steps must not keep rewinding to chapter one.
      const chapter = direction > 0 ? Math.ceil(position - .002) : direction < 0 ? Math.floor(position + .002) : Math.round(position)
      const top = start + distance * chapter / (count - 1)
      // A completed smooth scroll must not start another scroll loop.
      if (Math.abs(top - scroll) > 2) move(top)
    }, 180)
  }
  return { schedule, cancel }
}
