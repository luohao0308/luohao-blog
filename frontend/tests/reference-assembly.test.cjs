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

test('the additional article fan retains its independent original hybrid trajectory', () => {
  const { referenceFanFrames } = load('referenceAssembly.ts', { ASSEMBLY_REFERENCE_HTML: '' })
  assert.equal(referenceFanFrames(0, false)[1].transform, 'translate3d(calc(-50% + -160px),-62%,140px) rotateX(12deg) rotateZ(-11deg) scale(1)')
  assert.equal(referenceFanFrames(2, true)[2].transform, 'translate3d(calc(-50% + 38px),-65%,230px) rotateX(6deg) rotateZ(11deg)')
  assert.deepEqual(Array.from(referenceFanFrames(1, false), frame => frame.offset), [undefined, 0.28, 0.57, 0.88, undefined])
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
