import assert from 'node:assert/strict'

import { articleSummary, prepareArticleHtml, readingMinutes } from '../frontend/app/utils/article.ts'

assert.equal(readingMinutes(''), 0)
assert.equal(readingMinutes('中'.repeat(400)), 1)
assert.equal(readingMinutes('中'.repeat(401)), 2)
assert.equal(readingMinutes(Array(200).fill('word').join(' ')), 1)
assert.equal(readingMinutes(Array(201).fill('word').join(' ')), 2)

assert.equal(articleSummary('', '# 标题\n\n正文内容'), '标题 正文内容')
assert.equal(articleSummary(' 明确摘要 ', '正文'), '明确摘要')

const prepared = prepareArticleHtml([
  '<h2 id="existing">你好 世界</h2>',
  '<h2>你好 世界</h2>',
  '<h3>你好 世界-2</h3>',
  '<h3><code>API</code> / 边界</h3>',
  '<h2>&#99999999;</h2>',
].join(''))

assert.equal(prepared.headings[0].id, 'existing')
assert.equal(new Set(prepared.headings.map(heading => heading.id)).size, prepared.headings.length)
assert.equal(prepared.headings.at(-1).text, '&#99999999;')
assert.match(prepared.html, /<h2 id="existing">你好 世界<\/h2>/)

console.log('Article presentation utilities PASS')
