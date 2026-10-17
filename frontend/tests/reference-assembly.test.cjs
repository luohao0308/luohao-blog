const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const ts = require('typescript')
function load(file, dependency) {
  const exports = {}
  const code = ts.transpileModule(readFileSync(path.resolve(__dirname, '../app/utils', file), 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  vm.runInNewContext(code, { exports, Promise, setTimeout, clearTimeout, require: () => dependency })
  return exports
}
test('original desktop frame payload retains arrival, five-layer depth, opacity and none landing', () => {
  const { referenceLayerFrames } = load('referenceAssembly.ts', { ASSEMBLY_REFERENCE_HTML: '' })
  const frames = JSON.parse(JSON.stringify(referenceLayerFrames(0, false)))
  assert.deepEqual(frames, [
    { opacity: 0, transform: 'translate3d(-90px,-50px,-420px) rotateX(54deg) rotateY(-15deg) rotateZ(-5deg) scale(.72)' },
    { offset: 0.24, opacity: 1, transform: 'translate3d(0,-110px,80px) rotateX(36deg) rotateY(-11deg) rotateZ(-4deg) scale(.86)' },
    { offset: 0.60, opacity: 1, transform: 'translate3d(0,-110px,80px) rotateX(29deg) rotateY(8deg) rotateZ(2deg) scale(.88)' },
    { offset: 0.92, opacity: 1, transform: 'none' }, { opacity: 1, transform: 'none' },
  ])
  assert.match(referenceLayerFrames(4, false)[1].transform, /110px,-80px/)
})
test('original compact scene changes layer spread without reducing angles or scaling each panel', () => {
  const { referenceLayerFrames } = load('referenceAssembly.ts', { ASSEMBLY_REFERENCE_HTML: '' })
  assert.equal(referenceLayerFrames(0, true)[1].transform, 'translate3d(0,-52px,80px) rotateX(36deg) rotateY(-11deg) rotateZ(-4deg) scale(.86)')
  assert.equal(referenceLayerFrames(4, true)[2].transform, 'translate3d(0,52px,-80px) rotateX(29deg) rotateY(8deg) rotateZ(2deg) scale(.88)')
})

test('live assembly animates existing elements and lands at their real identity without creating a canvas', () => {
  const { createReferenceAssembly } = load('referenceAssembly.ts')
  const box = (left, top) => ({ left, right: left + 300, top, bottom: top + 180, width: 300, height: 180 })
  const nav = {}, footer = {}, layers = [{}, {}, {}, {}, {}]
  const cards = [0, 1, 2].map(i => ({ getBoundingClientRect: () => box(120 + i * 320, 650) }))
  const shell = { dataset: {}, querySelectorAll: selector => selector.includes('nav') ? [nav] : [footer] }
  const root = { closest: () => shell, querySelectorAll: selector => selector.includes('card') ? cards : layers, querySelector: () => ({ getBoundingClientRect: () => box(120, 100) }) }
  const scene = createReferenceAssembly(root, 1440)
  assert.deepEqual(Array.from(scene.steps, step => step.element), [nav, ...layers, footer, ...cards])
  assert.equal(shell.dataset.homeLiveAssembly, 'true')
  for (const step of scene.steps) {
    assert.equal(step.frames.at(-1).transform, 'none')
    assert.equal(step.frames.at(-2).transform, 'none')
    assert.equal(step.frames.at(-1).opacity, 1)
  }
  for (const step of scene.steps.slice(0, 7)) { assert.equal(step.duration, 2900); assert(step.delay <= 260) }
  for (const [index, step] of scene.steps.slice(7).entries()) { assert.equal(step.duration, 1900); assert.equal(step.delay, 530 + index * 65) }
  scene.cleanup(); assert.equal(shell.dataset.homeLiveAssembly, undefined)
})

function fixture(options = {}) {
  const env = { events: new EventTarget(), document: Object.assign(new EventTarget(), { hidden: !!options.hidden }), media: Object.assign(new EventTarget(), { matches: !!options.reduce }), memory: { value: false }, compact: false, viewportWidth: 1440, storage: () => ({ getItem: () => options.stored ? '1' : null, setItem() {} }) }
  const animations = []; let cleanups = 0, creations = 0
  const createReferenceAssembly = () => {
    creations++
    return { steps: Array.from({ length: 5 }, (_, index) => ({ element: { contains: () => false, animate(frames, timing) {
      if (options.throwAnimation) throw Error('WAAPI failure')
      let resolve, reject
      const animation = { frames, timing, playState: 'running', finished: new Promise((a, b) => { resolve = a; reject = b }), cancel() { reject(Error('cancelled')) }, resolve() { this.playState = 'finished'; resolve() }, reject() { reject(Error('interrupted')) } }
      animations.push(animation); return animation
    } }, frames: [{ opacity: 0 }, { opacity: 1 }], duration: 2900, delay: index * 65, easing: 'cubic-bezier(.25,.55,.25,1)' })), cleanup: () => cleanups++ }
  }
  return { env, root: { animate: options.unsupported ? undefined : () => {} }, animations, createReferenceAssembly, get cleanups() { return cleanups }, get creations() { return creations } }
}
test('reference controller preserves original timing and releases presentation once on all exits', async () => {
  for (const exit of ['skip', 'resize', 'finish', 'reject', 'throw']) {
    const f = fixture({ throwAnimation: exit === 'throw' })
    const { startHomeEntrance } = load('homeMotion.ts', { createReferenceAssembly: f.createReferenceAssembly })
    const controller = startHomeEntrance(f.root, f.env)
    if (exit === 'skip') controller.stop()
    if (exit === 'resize') f.env.events.dispatchEvent(new Event('resize'))
    if (exit === 'finish') f.animations.forEach(a => a.resolve())
    if (exit === 'reject') f.animations[0].reject()
    await new Promise(resolve => setImmediate(resolve))
    controller.stop()
    assert.equal(controller.playing, false); assert.equal(f.cleanups, 1)
    for (const [index, animation] of f.animations.entries()) {
      assert.equal(animation.timing.duration, 2900); assert.equal(animation.timing.delay, index * 65)
    }
    assert.equal(f.env.memory.value, exit !== 'throw')
  }
})
test('SSR fallbacks and already-seen sessions do not construct an overlay', () => {
  for (const options of [{ hidden: true }, { reduce: true }, { unsupported: true }, { stored: true }]) {
    const f = fixture(options)
    const { startHomeEntrance } = load('homeMotion.ts', { createReferenceAssembly: f.createReferenceAssembly })
    assert.equal(startHomeEntrance(f.root, f.env).playing, false); assert.equal(f.creations, 0)
    assert.equal(f.env.memory.value, false)
  }
})

test('the visible skip control can receive keyboard focus without being removed', () => {
  const f = fixture()
  const { startHomeEntrance } = load('homeMotion.ts', { createReferenceAssembly: f.createReferenceAssembly })
  const controller = startHomeEntrance(f.root, f.env)
  const focus = new Event('focusin')
  Object.defineProperty(focus, 'target', { value: { closest: selector => selector === '[data-home-skip]' ? {} : null } })
  f.env.events.dispatchEvent(focus)
  assert.equal(controller.playing, true); assert.equal(f.cleanups, 0)
  controller.stop(); assert.equal(f.cleanups, 1)
})
