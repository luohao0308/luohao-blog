const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const ts = require('typescript')
function load(name) {
  const exports = {}
  const source = readFileSync(path.resolve(__dirname, `../app/utils/${name}.ts`), 'utf8')
  vm.runInNewContext(ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText, { exports, require: () => load('assets') })
  return exports
}
test('reading assets map only backend image src paths deterministically for SSR', () => {
  const { readingAssetHtml } = load('reading')
  const input = '<img src="/v1/assets/a.png" alt="diagram"><img src="/api/v1/assets/b.png"><img src="https://example.com/a.png"><a href="/v1/assets/a.png">Original</a>'
  const expected = '<img src="/api/v1/assets/a.png" alt="diagram"><img src="/api/v1/assets/b.png"><img src="https://example.com/a.png"><a href="/v1/assets/a.png">Original</a>'
  assert.equal(readingAssetHtml(input), expected)
  assert.equal(readingAssetHtml(expected), expected)
})
