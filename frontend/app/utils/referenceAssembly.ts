export interface AssemblyStep {
  element: HTMLElement
  frames: Keyframe[]
  duration: number
  delay: number
  easing: string
}

// Keep the approved arrival, expansion and continuous pace; land on the actual
// elements at transform:none so completing the entrance never swaps documents.
export function referenceLayerFrames(index: number, compact: boolean): Keyframe[] {
  const spread = (index - 2) * (compact ? 26 : 55)
  const depth = (2 - index) * 40
  return [
    { opacity: 0, transform: `translate3d(${index % 2 ? 90 : -90}px,${spread + 60}px,-420px) rotateX(54deg) rotateY(-15deg) rotateZ(-5deg) scale(.72)` },
    { offset: 0.24, opacity: 1, transform: `translate3d(0,${spread}px,${depth}px) rotateX(36deg) rotateY(-11deg) rotateZ(-4deg) scale(.86)` },
    { offset: 0.60, opacity: 1, transform: `translate3d(0,${spread}px,${depth}px) rotateX(29deg) rotateY(8deg) rotateZ(2deg) scale(.88)` },
    { offset: 0.92, opacity: 1, transform: 'none' },
    { opacity: 1, transform: 'none' },
  ]
}


export function createReferenceAssembly(root: HTMLElement, viewportWidth: number) {
  const shell = root.closest<HTMLElement>('[data-home-shell]') ?? root
  const compact = viewportWidth < 600
  const layers = [
    ...shell.querySelectorAll<HTMLElement>('[data-home-shell-layer="nav"]'),
    ...root.querySelectorAll<HTMLElement>('[data-home-layer]'),
    ...shell.querySelectorAll<HTMLElement>('[data-home-shell-layer="footer"]'),
  ]
  const cards = [...root.querySelectorAll<HTMLElement>('[data-home-card]')].slice(0, 3)
  const rectangles = cards.map(card => card.getBoundingClientRect())
  const hero = root.querySelector<HTMLElement>('[data-home-hero]')?.getBoundingClientRect()
  const first = rectangles[0]
  const multiRow = !!first && rectangles.some(rect => Math.abs(rect.top - first.top) > 1)
  const anchorX = first ? (Math.min(...rectangles.map(rect => rect.left)) + Math.max(...rectangles.map(rect => rect.right))) / 2 : 0
  const anchorY = first ? (hero?.bottom ?? first.top) + first.height * 0.22 : 0
  const previous = shell.dataset.homeLiveAssembly
  shell.dataset.homeLiveAssembly = 'true'
  const cleanup = () => {
    if (previous === undefined) delete shell.dataset.homeLiveAssembly
    else shell.dataset.homeLiveAssembly = previous
  }
  const steps: AssemblyStep[] = layers.map((element, position) => {
    const index = Math.min(position, 4)
    const frames = referenceLayerFrames(index, compact).map(frame => ({
      ...frame,
      transform: frame.transform === 'none' ? 'none' : `perspective(1800px) ${frame.transform}`,
    }))
    return { element, frames, duration: 2900, delay: index * 65, easing: 'cubic-bezier(.25,.55,.25,1)' }
  })
  cards.forEach((element, index) => {
    const rect = rectangles[index]!
    const side = index - (cards.length - 1) / 2
    const gap = compact ? Math.min(36, rect.width * 0.09) : multiRow ? 120 : 155
    const x = anchorX + side * gap - (rect.left + rect.width / 2)
    const y = anchorY - (rect.top + rect.height / 2) + Math.abs(side) * (compact ? 5 : 12)
    const scale = compact ? 0.62 : multiRow ? 0.72 : 0.92
    const pose = `perspective(1800px) translate3d(${x}px,${y}px,${(45 + index * 45) * (compact ? 0.45 : 1)}px) rotateX(${compact ? 4.5 : 10}deg) rotateZ(${side * (compact ? 4.5 : 10)}deg) scale(${scale})`
    steps.push({ element, duration: 1900, delay: 530 + index * 65, easing: 'cubic-bezier(.25,.55,.25,1)', frames: [
      { transform: 'none', opacity: 1 },
      { offset: 0.28, transform: pose, opacity: 1 },
      { offset: 0.57, transform: pose, opacity: 1 },
      { offset: 0.92, transform: 'none', opacity: 1 },
      { transform: 'none', opacity: 1 },
    ] })
  })
  return { steps, cleanup }
}
