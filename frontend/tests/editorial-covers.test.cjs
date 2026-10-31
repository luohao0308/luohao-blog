const { test } = require('node:test')
const assert = require('node:assert/strict')
const { readFileSync, existsSync } = require('node:fs')
const vm = require('node:vm')
const ts = require('typescript')

test('approved article covers stay independent from generated project covers', () => {
  const exports = {}
  vm.runInNewContext(ts.transpileModule(readFileSync('app/data/editorial.ts', 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText, { exports })
  const expected = {
    'ai-delivery-guard': 'guard', 'fullstack-smoke-pitfalls': 'fullstack',
    'ent-m2m-edge-case-study': 'ent', 'mysql-varchar-191-case-study': 'mysql',
    'dev-workflow-project': 'workflow', 'forge-harness-project': 'harness', 'aigc-backend-delivery': 'aigc',
  }
  for (const [slug, name] of Object.entries(expected)) {
    const cover = exports.articleImages[slug]
    assert.equal(cover.src, `/images/editorial/covers/${name}.webp`)
    assert.equal(cover.poster, true)
    assert.ok(existsSync('public' + cover.src))
  }
  const articlePaths = new Set(Object.values(exports.articleImages).map(image => image.src))
  for (const cover of Object.values(exports.projectImages)) {
    assert.ok(cover.src.startsWith('/images/projects/generated/'))
    assert.ok(!articlePaths.has(cover.src))
    assert.ok(existsSync('public' + cover.src))
  }
})
