// Read-only local backend for SSR browser fixtures. Never connects to real data.
const http = require('node:http')
const pixelPng = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aD1sAAAAASUVORK5CYII=', 'base64')
function makeArticle(overrides = {}) {
  return {
    id: '1', slug: 'fixture-article', title: '并发与一致性', summary: '分布式请求处理',
    content_md: '## 请求处理\n\n示例正文',
    content_html: '<h2>请求处理</h2><p>分布式请求处理</p><h2>请求处理</h2><h3>重试</h3><pre><code class="language-go">fmt.Println("hello")\n</code></pre><img src="/api/v1/assets/fixture.png" alt="架构示例">',
    status: 2, tags: ['Go'], view_count: 3, like_count: 0, created_at: '2026-10-01T00:00:00Z', updated_at: '2026-10-01T00:00:00Z', published_at: '2026-10-01T00:00:00Z',
    ...overrides,
  }
}
async function startFixture(options = {}) {
  const articles = options.articles ?? [makeArticle()]
  const server = http.createServer((req, res) => {
    const pathname = new URL(req.url, 'http://127.0.0.1:13001').pathname
    res.setHeader('Content-Type', 'application/json; charset=utf-8')
    const json = (value, status = 200) => { res.statusCode = status; res.end(JSON.stringify(value)) }
    if (req.method !== 'GET') return json({ message: 'Read-only fixture' }, 405)
    if (pathname === '/v1/assets/fixture.png') { res.setHeader('Content-Type', 'image/png'); return res.end(pixelPng) }
    if (pathname.includes('/auth/')) return json({ message: 'Anonymous fixture' }, 401)
    if (pathname === '/v1/articles/list' || pathname === '/v1/search/articles') return json({ articles })
    if (/\/articles\/[^/]+\/comments$/.test(pathname)) return json({ comments: [], total: 0 })
    if (pathname === '/v1/collections/list') return json({ collections: [] })
    if (pathname.startsWith('/v1/articles/')) {
      const article = articles.find(item => item.slug === decodeURIComponent(pathname.split('/')[3]))
      return article ? json(article) : json({ message: 'Not found' }, 404)
    }
    if (pathname.includes('/projects')) return json({ projects: [] })
    if (pathname.includes('/categories')) return json({ categories: [] })
    return json({ message: `Unknown fixture route: ${pathname}` }, 404)
  })
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(Number(process.env.BLOG_E2E_FIXTURE_PORT || 13001), '127.0.0.1', resolve) })
  return { articles, close: () => new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve())) }
}
module.exports = { startFixture, makeArticle, pixelPng }
