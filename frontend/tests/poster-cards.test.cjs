const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const { test } = require('node:test')
const vm = require('node:vm')
const ts = require('typescript')
const { computed, ref, reactive } = require('vue')

for (const [name, entity, mapping] of [['ArticleCard', 'article', 'articleImages'], ['ProjectCard', 'project', 'projectImages']]) {
  test(`${name}: explicit preview, editorial poster, DB fallback and failed image`, () => {
    const props = reactive({ [entity]: { slug: 'mapped', title: 'Title', name: 'Project', cover_url: '' } })
    const mounted = []
    const source = readFileSync(`app/components/${name}.vue`, 'utf8')
    const script = source.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1].replace(/^import .*$/gm, '')
    const context = { computed, ref, defineProps: () => props, onMounted: fn => mounted.push(fn), useArticleExpansion: () => () => {}, assetUrl: v => v || '', articleSummary: () => '', [mapping]: { mapped: { src: '/poster.webp', alt: 'Poster alt', poster: true } } }
    vm.runInNewContext(ts.transpileModule(script + '\nglobalThis.result = { coverUrl, isPoster, alt, showCover, failedCover, coverImage }', { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context)
    const r = context.result
    assert.equal(r.coverUrl.value, '/poster.webp')
    assert.equal(r.isPoster.value, true)
    assert.equal(r.alt.value, 'Poster alt')
    assert.equal(Boolean(r.showCover.value), true)
    props.coverSrc = '/preview.jpg'
    props.coverAlt = 'Preview alt'
    assert.equal(r.coverUrl.value, '/preview.jpg')
    assert.equal(r.isPoster.value, false)
    assert.equal(r.alt.value, 'Preview alt')
    props.coverSrc = undefined
    props.coverAlt = undefined
    r.coverImage.value = { complete: true, naturalWidth: 0 }
    mounted.forEach(fn => fn())
    assert.equal(r.showCover.value, false)
    props[entity].cover_url = '/db.jpg'
    assert.equal(r.coverUrl.value, '/db.jpg')
    assert.equal(Boolean(r.isPoster.value), false)
    props[entity].slug = 'unmapped'
    assert.equal(r.coverUrl.value, '/db.jpg')
    assert.equal(Boolean(r.isPoster.value), false)
    assert.equal(Boolean(r.showCover.value), true)
    r.failedCover.value = '/db.jpg'
    assert.equal(r.showCover.value, false)
    assert.match(source, /data-(article|project)-card/)
    assert.match(source, /@click.capture="open(Article|Project)/)
  })
}

test('curated SEO covers stay on the explicit local allowlist', () => {
  const source = readFileSync('app/utils/seo.ts', 'utf8')
  const exports = {}
  vm.runInNewContext(ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText, { exports })
  assert.equal(exports.publicAssetPath('/v1/assets/media/' + 'a'.repeat(32) + '.webp'), '/api/v1/assets/media/' + 'a'.repeat(32) + '.webp')
  assert.equal(exports.publicAssetPath('/images/editorial/covers/workflow.webp'), '/images/editorial/covers/workflow.webp')
  for (const bad of ['https://example.com/a.webp', '/images/editorial/covers/../secret.webp', '/images/editorial/covers/unknown.webp'])
    assert.equal(exports.publicAssetPath(bad), exports.DEFAULT_SEO_IMAGE)
})
