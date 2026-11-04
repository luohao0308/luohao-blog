const assert = require('node:assert/strict')
const { test } = require('node:test')
const { readFileSync } = require('node:fs')
const vm = require('node:vm')
const ts = require('typescript')
const { ref, computed, watch, effectScope, nextTick } = require('vue')

function fixture() {
  const raf = new Map(), listeners = new Map(), cleanups = [], captures = new Set(), effects = effectScope()
  let sequence = 0, geometryReads = 0
  const element = { getBoundingClientRect: () => { geometryReads++; return { left: 200, top: 300, width: 88, height: 96 } }, setPointerCapture: id => captures.add(id), releasePointerCapture: id => captures.delete(id), hasPointerCapture: id => captures.has(id) }
  const trigger = ref(element), enabled = ref(true), exports = {}
  const viewport = { innerWidth: 400, innerHeight: 600, addEventListener: (n, fn) => listeners.set(n, fn), removeEventListener: n => listeners.delete(n) }
  const context = { exports, ref, computed, watch, Math, window: viewport, onMounted: fn => fn(), onBeforeUnmount: fn => cleanups.push(fn), requestAnimationFrame: fn => { raf.set(++sequence, fn); return sequence }, cancelAnimationFrame: id => raf.delete(id) }
  effects.run(() => vm.runInNewContext(ts.transpileModule(readFileSync('app/composables/useMascotDrag.ts', 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText, context))
  const api = effects.run(() => exports.useMascotDrag(trigger, enabled))
  const pointer = (x, y, extras = {}) => ({ clientX: x, clientY: y, pointerId: 1, isPrimary: true, button: 0, preventDefault() {}, ...extras })
  return { ...api, enabled, viewport, captures, raf, listeners, pointer, get geometryReads() { return geometryReads }, flush() { for (const [id, fn] of [...raf]) { raf.delete(id); fn() } }, close() { cleanups.forEach(fn => fn()); effects.stop() } }
}
const point = value => JSON.parse(JSON.stringify(value))
const click = detail => ({ detail, preventDefault() { this.prevented = true }, stopImmediatePropagation() {} })

test('tap threshold preserves ordinary click and ignores secondary input', () => {
  const f = fixture()
  f.onPointerDown(f.pointer(210, 310, { button: 2 })); assert.equal(f.captures.size, 0)
  f.onPointerDown(f.pointer(210, 310)); f.onPointerMove(f.pointer(213, 313)); f.onPointerUp(f.pointer(213, 313))
  assert.equal(f.position.value, null); assert.equal(f.suppressClick(click(1)), false); assert.equal(f.captures.size, 0)
  f.close()
})
test('drag merges RAF writes, flushes release and suppresses only the drag click', () => {
  const f = fixture()
  f.onPointerDown(f.pointer(210, 310)); f.onPointerMove(f.pointer(230, 330)); f.onPointerMove(f.pointer(250, 340))
  assert.equal(f.geometryReads, 1); assert.equal(f.raf.size, 1); assert.equal(f.dragging.value, true); assert.equal(f.position.value, null)
  f.flush(); assert.deepEqual(point(f.position.value), { x: 240, y: 330 })
  f.onPointerUp(f.pointer(260, 350)); assert.deepEqual(point(f.position.value), { x: 250, y: 340 })
  assert.equal(f.raf.size, 0); assert.equal(f.dragging.value, false); assert.equal(f.suppressClick(click(1)), true)
  assert.equal(f.suppressClick(click(0)), false)
  f.onPointerDown(f.pointer(260, 350)); f.onPointerUp(f.pointer(260, 350)); assert.equal(f.suppressClick(click(1)), false)
  f.close()
})
test('cancel and lost capture discard uncommitted movement and never open', () => {
  for (const method of ['onPointerCancel', 'onLostCapture']) {
    const f = fixture(); f.onPointerDown(f.pointer(210, 310)); f.onPointerMove(f.pointer(250, 350)); f.flush()
    f.onPointerMove(f.pointer(280, 380)); f[method](f.pointer(280, 380)); f.flush()
    assert.deepEqual(point(f.position.value), { x: 240, y: 340 }); assert.equal(f.raf.size, 0)
    assert.equal(f.suppressClick(click(1)), true); assert.equal(f.captures.size, 0); f.close()
  }
})
test('keyboard and resize clamp coordinates; disabling and unmount clean resources', async () => {
  const f = fixture(); let prevented = false
  f.onKeydown({ key: 'ArrowRight', preventDefault() { prevented = true } })
  assert(prevented); assert.deepEqual(point(f.position.value), { x: 216, y: 300 })
  f.viewport.innerWidth = 180; f.viewport.innerHeight = 200; f.listeners.get('resize')()
  assert.deepEqual(point(f.position.value), { x: 76, y: 88 })
  f.onPointerDown(f.pointer(100, 100)); f.onPointerMove(f.pointer(120, 120)); f.enabled.value = false; await nextTick()
  assert.equal(f.raf.size, 0); assert.equal(f.captures.size, 0); assert.equal(f.dragging.value, false)
  assert.equal(f.suppressClick(click(0)), false)
  f.onPointerDown(f.pointer(100, 100)); assert.equal(f.suppressClick(click(1)), false)
  f.close(); assert.equal(f.listeners.size, 0)
  const active = fixture(); active.onPointerDown(active.pointer(210, 310)); active.onPointerMove(active.pointer(230, 330))
  active.close(); assert.equal(active.raf.size, 0); assert.equal(active.captures.size, 0); assert.equal(active.listeners.size, 0)
})

test('source becoming ready during a fallback tap does not suppress its first click', async () => {
  const f = fixture()
  f.enabled.value = false; await nextTick()
  f.onPointerDown(f.pointer(210, 310))
  assert.equal(f.captures.size, 0)
  f.enabled.value = true; await nextTick()
  f.onPointerUp(f.pointer(210, 310))
  assert.equal(f.suppressClick(click(1)), false)
  assert.equal(f.position.value, null)
  f.close()
})
