const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const ts = require('typescript')
function fixture() {
  const pending = new Map(), calls = [], api = {}
  let next = 0, geometry = { scroll: 660, start: 100, distance: 4000, count: 5 }
  vm.runInNewContext(ts.transpileModule(readFileSync(path.join(__dirname, '../app/utils/aboutJourneySnap.ts'), 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText, {
    exports: api, setTimeout: fn => { pending.set(++next, fn); return next }, clearTimeout: id => pending.delete(id),
  })
  const snap = api.createJourneyScrollSnap(() => geometry, top => calls.push(top))
  return { snap, calls, pending, set: value => { geometry = value }, flush() { const work = [...pending.values()]; pending.clear(); work.forEach(fn => fn()) } }
}
test('scroll idle completes the gesture direction rather than leaving two cropped scenes', () => {
  const f = fixture()
  f.snap.schedule(); f.flush(); assert.deepEqual(f.calls, [1100])
  f.set({ scroll: 2100, start: 100, distance: 4000, count: 5 }); f.snap.schedule(); f.flush()
  f.set({ scroll: 1540, start: 100, distance: 4000, count: 5 }); f.snap.schedule(); f.flush()
  assert.deepEqual(f.calls, [1100, 1100]) // reverse gesture completes back to the previous chapter
})
test('continued input coalesces timers and settles using the latest geometry', () => {
  const f = fixture()
  f.snap.schedule(); f.snap.schedule(); assert.equal(f.pending.size, 1)
  f.set({ scroll: 1770, start: 100, distance: 4000, count: 5 }); f.flush(); assert.deepEqual(f.calls, [2100])
})
test('reading/reduced modes, page boundaries and completed smooth scroll never trigger a snap', () => {
  const f = fixture()
  for (const geometry of [null, { scroll: 20, start: 100, distance: 4000, count: 5 }, { scroll: 4200, start: 100, distance: 4000, count: 5 }, { scroll: 1099, start: 100, distance: 4000, count: 5 }]) {
    f.set(geometry); f.snap.schedule(); f.flush()
  }
  assert.deepEqual(f.calls, [])
})
test('cancel removes pending work on resize, mode changes or unmount', () => {
  const f = fixture(); f.snap.schedule(); f.snap.cancel(); f.flush(); assert.equal(f.pending.size, 0); assert.deepEqual(f.calls, [])
})

test('a short wheel gesture advances instead of repeatedly rewinding to the same chapter', () => {
  const f = fixture(); f.set({ scroll: 180, start: 100, distance: 4000, count: 5 })
  f.snap.schedule(); f.flush(); assert.deepEqual(f.calls, [1100])
})

test('subpixel completion tolerance does not cascade into another chapter on short viewports', () => {
  const f = fixture(); f.set({ scroll: 201, start: 100, distance: 400, count: 5 })
  f.snap.schedule(); f.flush(); assert.deepEqual(f.calls, [])
})
