import assert from 'node:assert/strict'
import { DEFAULT_SEO_IMAGE, publicAssetPath, siteOrigin, siteURL, sitemapLastmod, xmlEscape } from '../frontend/app/utils/seo.ts'

assert.equal(siteOrigin('https://makerhao.cn/path?tracking=1'), 'https://makerhao.cn')
for (const origin of ['javascript:alert(1)', 'https://user:password@example.com', 'file:///tmp/test']) assert.throws(() => siteOrigin(origin))
assert.equal(siteURL('https://makerhao.cn', '/posts/hello'), 'https://makerhao.cn/posts/hello')
assert.equal(siteURL('https://makerhao.cn', '/tags/中文'), 'https://makerhao.cn/tags/%E4%B8%AD%E6%96%87')
// Route/media input cannot turn a trusted canonical origin into another host.
for (const path of ['//evil.example/path', '\\evil.example/path', '/\\evil.example/path']) {
  try { assert.equal(new URL(siteURL('https://makerhao.cn', path)).origin, 'https://makerhao.cn') }
  catch (error) { if (error.code === 'ERR_ASSERTION') throw error }
}
assert.throws(() => siteURL('https://makerhao.cn', '/posts/test\r\nheader'))
assert.equal(publicAssetPath('https://evil.example/a.png'), DEFAULT_SEO_IMAGE)
assert.equal(publicAssetPath('//evil.example/a.png'), DEFAULT_SEO_IMAGE)
assert.equal(publicAssetPath('/v1/assets/media/../../unsafe.png'), DEFAULT_SEO_IMAGE)
assert.equal(publicAssetPath(`/v1/assets/media/${'a'.repeat(32)}.png`), `/api/v1/assets/media/${'a'.repeat(32)}.png`)
assert.equal(xmlEscape('<&>"\''), '&lt;&amp;&gt;&quot;&apos;')
assert.equal(sitemapLastmod('invalid'), undefined)
assert.equal(sitemapLastmod({ seconds: 'not-a-number' }), undefined)
assert.equal(sitemapLastmod('2026-10-12T08:00:00Z'), '2026-10-12T08:00:00.000Z')
console.log('SEO utility PASS: trusted origin, controlled assets, XML escaping, timestamps')
