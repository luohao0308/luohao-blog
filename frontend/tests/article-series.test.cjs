const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const ts = require('typescript')
const source = readFileSync(path.resolve(__dirname, '../app/data/articleSeries.ts'), 'utf8')
const api = {}
vm.runInNewContext(ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText, { exports: api })
const series = { slug: 'test', articleSlugs: ['first', 'private', 'missing', 'last', 'first'] }
const articles = [{ slug: 'last', status: 2 }, { slug: 'private', status: 1 }, { slug: 'first', status: 2 }, { slug: 'deleted', status: 3 }]
test('series order ignores API order, excludes private/missing entries and deduplicates', () => {
  assert.equal(JSON.stringify(api.seriesArticles(series, articles).map(a => a.slug)), JSON.stringify(['first', 'last']))
  assert.equal(api.seriesArticles(series, []).length, 0)
})
test('series position counts public entries and respects first/last boundaries', () => {
  const first = api.seriesPosition(series, articles, 'first')
  assert.equal(first.index, 0)
  assert.equal(first.total, 2)
  assert.equal(first.previous, undefined)
  assert.equal(first.next.slug, 'last')
  const last = api.seriesPosition(series, articles, 'last')
  assert.equal(last.index, 1)
  assert.equal(last.previous.slug, 'first')
  assert.equal(last.next, undefined)
  assert.equal(api.seriesPosition(series, articles, 'private'), undefined)
  assert.equal(api.seriesPosition(series, articles, 'missing'), undefined)
})
test('configured series are unique and all article slugs are explicit', () => {
  assert.equal(new Set(api.articleSeries.map(s => s.slug)).size, api.articleSeries.length)
  for (const s of api.articleSeries) {
    assert.ok(s.title && s.description)
    assert.equal(new Set(s.articleSlugs).size, s.articleSlugs.length)
  }
})

test('series keep independent accessible covers and nonempty reading topics', () => {
  assert.equal(new Set(api.articleSeries.map(s => s.cover)).size, api.articleSeries.length)
  for (const s of api.articleSeries) {
    assert.equal(s.cover, `/images/series/${s.slug}.svg`)
    assert.ok(s.coverAlt.includes(s.title))
    assert.ok(s.topics.length > 0 && s.topics.every(topic => typeof topic === 'string' && topic.trim()))
  }
})
