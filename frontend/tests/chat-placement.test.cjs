const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs'), vm = require('node:vm'), ts = require('typescript')
const exportsObject = {}
vm.runInNewContext(ts.transpileModule(fs.readFileSync('app/utils/chatPlacement.ts', 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText, { exports: exportsObject })
const { chatPlacement } = exportsObject
test('bottom and top pets open in usable areas while contacts remain reachable', () => {
  const bottom = chatPlacement({ left: 1336, right: 1416, top: 780, bottom: 876 }, 1440, 900)
  assert.equal(bottom.style.top, 'auto'); assert.equal(bottom.style.bottom, '132px')
  const top = chatPlacement({ left: 16, right: 96, top: 16, bottom: 112 }, 390, 900)
  assert.equal(top.style.top, '124px'); assert.equal(top.style.bottom, 'auto')
  assert.equal(top.clearance, '88px')
})
test('short screens use viewport height instead of an inaccessible thin panel', () => {
  const p = chatPlacement({ left: 180, right: 244, top: 160, bottom: 230 }, 320, 360)
  assert.equal(p.style.top, '16px'); assert.equal(p.style.maxHeight, '328px')
})
test('placement stays within viewport bounds at drag positions and enlarged text sizes', () => {
  for (const width of [320, 390, 1024, 1440]) for (const height of [240, 360, 480, 900]) for (const unit of [16, 20]) {
    for (const left of [16, width / 2 - 40, width - 96]) for (const top of [16, height / 2 - 44, height - 112]) {
      const p = chatPlacement({ left, right: left + 80, top, bottom: top + 96 }, width, height, unit)
      assert(Number.parseFloat(p.style.left) >= unit)
      assert(Number.parseFloat(p.style.left) + Math.min(28 * unit, width - 2 * unit) <= width - unit + .01)
      const max = Number.parseFloat(p.style.maxHeight)
      assert(max >= 0 && max <= height - 2 * unit)
      if (p.style.top !== 'auto') assert(Number.parseFloat(p.style.top) + max <= height - unit + .01)
      else assert(height - Number.parseFloat(p.style.bottom) - max >= unit - .01)
    }
  }
})
