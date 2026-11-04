const assert = require('node:assert/strict')
const { test } = require('node:test')
const { readFileSync } = require('node:fs')
const vm = require('node:vm')
const ts = require('typescript')
const { ref, watch, effectScope, nextTick } = require('vue')

function fixture() {
  const ready = ref(true), opened = ref(false), dragging = ref(false), position = ref({ x: 100, y: 100 })
  const timers = new Map(), listeners = new Map(), cleanups = [], mounts = [], effects = effectScope()
  let clock = 0, sequence = 0
  const media = { matches: false, addEventListener: (name, fn) => listeners.set('media:' + name, fn), removeEventListener: name => listeners.delete('media:' + name) }
  const document = { hidden: false, addEventListener: (name, fn) => listeners.set(name, fn), removeEventListener: name => listeners.delete(name) }
  const exports = {}
  const context = { exports, ref, watch, document, window: { matchMedia: () => media }, Math: { ...Math, floor: Math.floor, random: () => 0 },
    setTimeout(fn, delay) { timers.set(++sequence, { fn, at: clock + delay }); return sequence }, clearTimeout: id => timers.delete(id),
    onMounted: fn => mounts.push(fn), onBeforeUnmount: fn => cleanups.push(fn) }
  effects.run(() => vm.runInNewContext(ts.transpileModule(readFileSync('app/composables/usePetActivity.ts', 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText, context))
  const api = effects.run(() => exports.usePetActivity(ready, opened, dragging, position))
  mounts.forEach(fn => fn())
  return { ...api, ready, opened, dragging, position, timers, listeners, media, document,
    tick(ms) { const end = clock + ms; let entry; while ((entry = [...timers].sort((a, b) => a[1].at - b[1].at).find(([, timer]) => timer.at <= end))) { const [id, timer] = entry; timers.delete(id); clock = timer.at; timer.fn() } clock = end },
    close() { cleanups.forEach(fn => fn()); effects.stop() } }
}

test('idle actions wait twelve seconds, rotate briefly, and return to idle', () => {
  const f = fixture()
  assert.equal(f.state.value, 'idle'); f.tick(11999); assert.equal(f.state.value, 'idle')
  for (const expected of ['wave', 'jump', 'waiting']) {
    f.tick(expected === 'wave' ? 1 : 12000); assert.equal(f.state.value, expected)
    f.tick(1600); assert.equal(f.state.value, 'idle')
  }
  f.close(); assert.equal(f.timers.size, 0); assert.equal(f.listeners.size, 0)
})

test('opened panel suppresses idle timers; only request status drives working and completion', async () => {
  const f = fixture(); f.opened.value = true; await nextTick()
  assert.equal(f.state.value, 'waiting'); assert.equal(f.timers.size, 0)
  f.tick(60000); assert.equal(f.state.value, 'waiting')
  f.onRequestStatus('pending'); assert.equal(f.state.value, 'working')
  f.tick(2500); assert.equal(f.state.value, 'reviewing')
  f.tick(2500); assert.equal(f.state.value, 'waiting')
  f.greet(); assert.equal(f.state.value, 'waiting')
  f.onRequestStatus('done'); assert.equal(f.state.value, 'wave')
  f.tick(1800); assert.equal(f.state.value, 'waiting'); assert.equal(f.timers.size, 0)
  f.onRequestStatus('failure'); assert.equal(f.state.value, 'failure')
  f.tick(1800); assert.equal(f.state.value, 'waiting'); f.close()
})

test('drag takes priority, follows horizontal movement, and resumes pending after release', async () => {
  const f = fixture(); f.onRequestStatus('pending')
  f.dragging.value = true; await nextTick(); assert.equal(f.state.value, 'jump'); assert.equal(f.timers.size, 0)
  f.position.value = { x: 120, y: 100 }; await nextTick(); assert.equal(f.state.value, 'move-right')
  f.position.value = { x: 90, y: 100 }; await nextTick(); assert.equal(f.state.value, 'move-left')
  f.tick(60000); assert.equal(f.state.value, 'move-left')
  f.dragging.value = false; await nextTick(); assert.equal(f.state.value, 'working')
  f.onRequestStatus('idle'); assert.equal(f.state.value, 'idle'); f.close()
})

test('hidden, reduced motion and unready pets stop clocks; resume restarts scheduling', async () => {
  const f = fixture(); f.onRequestStatus('pending')
  f.document.hidden = true; f.listeners.get('visibilitychange')()
  assert.equal(f.state.value, 'idle'); assert.equal(f.timers.size, 0)
  f.tick(60000); assert.equal(f.state.value, 'idle')
  f.document.hidden = false; f.listeners.get('visibilitychange')(); assert.equal(f.state.value, 'working')
  f.media.matches = true; f.listeners.get('media:change')(); assert.equal(f.state.value, 'idle'); assert.equal(f.timers.size, 0)
  f.media.matches = false; f.listeners.get('media:change')(); assert.equal(f.timers.size, 1)
  f.ready.value = false; await nextTick(); assert.equal(f.state.value, 'idle'); assert.equal(f.timers.size, 0)
  f.ready.value = true; await nextTick(); f.tick(2500); assert.equal(f.state.value, 'reviewing')
  f.close(); assert.equal(f.timers.size, 0); assert.equal(f.listeners.size, 0)
})


test('greeting never overrides or prolongs completion and failure feedback', async () => {
  for (const [status, expected] of [['done', 'wave'], ['failure', 'failure']]) {
    const f = fixture(); f.opened.value = true; await nextTick()
    f.onRequestStatus(status); f.tick(1000)
    f.greet(); assert.equal(f.state.value, expected)
    f.tick(799); assert.equal(f.state.value, expected)
    f.tick(1); assert.equal(f.state.value, 'waiting')
    assert.equal(f.timers.size, 0); f.close()
  }
})
