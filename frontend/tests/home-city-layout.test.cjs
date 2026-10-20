const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const ts = require('typescript')
const source = readFileSync(path.resolve(__dirname, '../app/utils/backgroundLabCityVariants.ts'), 'utf8')
const api = {}
vm.runInNewContext(ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText, { exports: api, require: () => ({}) })
test('hero city projected bounds fit and remain vertically centered at mobile and desktop sizes', () => {
  for (const [width, height] of [[282, 320], [342, 320], [440, 440 / 1.3], [580, 580 / 1.3], [650, 650 / 1.3]]) {
    const { scale, centerX, centerY } = api.businessCityViewport(width, height, 'hero')
    const left = centerX - 620 * scale, right = centerX + 620 * scale
    const top = centerY - 470 * scale, bottom = centerY + 320 * scale
    assert.ok(left >= 7 && right <= width - 7)
    assert.ok(top >= 0 && bottom <= height - 48)
    assert.ok(right - left >= width * .85)
    assert.ok(Math.abs((top + bottom) / 2 - (height - 48) / 2) < .001)
  }
})
test('background placement remains unchanged', () => {
  for (const [w, h] of [[390, 844], [1440, 900]]) {
    const value = api.businessCityViewport(w, h)
    const scale = w < 600 ? w * .9 / 1240 : Math.min(w * .66 / 1240, h * .83 / 720)
    assert.equal(value.scale, scale)
    assert.equal(value.centerY, h * (w < 600 ? .67 : .63))
    assert.equal(value.centerX, w < 600 ? w * .5 : Math.min(w * .69, w - 1240 * .52 * scale - 18))
  }
})
test('homepage uses transparent hero canvas and a shared viewport aircraft', () => {
  const visual = readFileSync(path.resolve(__dirname, '../app/components/HomeVisual.vue'), 'utf8')
  const page = readFileSync(path.resolve(__dirname, '../app/pages/index.vue'), 'utf8')
  assert.match(visual, /:transparent="true"/)
  assert.match(visual, /aircraft: false, layout: 'hero'/)
  assert.doesNotMatch(visual, /ctx\.fillRect|border-radius: 16px/)
  assert.match(page, /AboutAircraftOverlay/)
  assert.match(page, /:show-controls="false"/)
  assert.doesNotMatch(visual, /home-city-tools|暂停城市|重置城市/)
})

test('hero sun sits above the skyline with room for its rays; background keeps its original position', () => {
  for (const [w, h] of [[282, 320], [440, 338], [580, 446]]) {
    const sun = api.businessCitySunPosition(w, h, 'hero')
    assert.ok(sun.y >= 32 && sun.y < h * .2)
    assert.equal(sun.x, w * .87)
    assert.equal(api.businessCitySunPosition(w, h).y, h * (w < 600 ? .51 : .3))
  }
})
