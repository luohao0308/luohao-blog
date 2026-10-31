// Local-only preview: fresh public snapshots plus a review proposal. No CMS writes.
const http = require('node:http')
const fs = require('node:fs')
const path = require('node:path')
const root = path.resolve(__dirname, '../..')
function createFixture({ revised = true } = {}) {
  const baseline = path.join(root, 'output/current-reading/live-baseline')
  const read = name => JSON.parse(fs.readFileSync(path.join(baseline, name + '.json'), 'utf8'))
  const projects = read('projects').projects
  const articles = read('articles').articles
  const about = read('about')
  if (revised) {
    const proposal = JSON.parse(fs.readFileSync(path.join(root, 'docs/content/portfolio-revision-2026-10.json'), 'utf8'))
    const html = JSON.parse(fs.readFileSync(path.join(root, 'output/current-reading/rendered-articles.json'), 'utf8'))
    for (const [kind, records] of [['projects', projects], ['articles', articles]]) {
      for (const change of proposal[kind]) {
        const record = records.find(item => item.slug === change.slug)
        if (!record || record.id !== change.id || record.updated_at !== change.expectedUpdatedAt) throw new Error('CMS baseline changed: ' + change.slug)
        for (const [key, value] of Object.entries(change.before)) {
          if (JSON.stringify(record[key]) !== JSON.stringify(value)) throw new Error('CMS field changed: ' + change.slug + '.' + key)
        }
        Object.assign(record, change.after)
        if (kind === 'articles') record.content_html = html[change.slug]
      }
    }
  }
  const server = http.createServer((req, res) => {
    res.setHeader('Content-Type', 'application/json')
    const url = new URL(req.url, 'http://fixture')
    const reply = (data, status = 200) => { res.statusCode = status; res.end(JSON.stringify(data)) }
    if (url.pathname.includes('/auth/')) return reply({}, 401)
    if (url.pathname.endsWith('/about')) return reply(about)
    if (url.pathname.includes('/collections')) return reply({ collections: [], next_page_token: '' })
    if (url.pathname.endsWith('/projects/list')) return reply({ projects, next_page_token: '' })
    if (url.pathname.endsWith('/articles/list')) return reply({ articles, next_page_token: '' })
    if (url.pathname.includes('/search/articles')) {
      const query = (url.searchParams.get('query') || '').toLowerCase()
      return reply({ articles: articles.filter(a => `${a.title} ${a.summary}`.toLowerCase().includes(query)), next_page_token: '' })
    }
    for (const [kind, records] of [['projects', projects], ['articles', articles]]) {
      const slug = url.pathname.match(new RegExp('/' + kind + '/([^/]+)$'))?.[1]
      if (slug) return reply(records.find(item => item.slug === decodeURIComponent(slug)) || {}, records.some(item => item.slug === decodeURIComponent(slug)) ? 200 : 404)
    }
    return reply({ comments: [], tags: [], categories: [], next_page_token: '' })
  })
  return { server, projects, articles }
}
module.exports = { createFixture }
if (require.main === module) {
  const { spawn } = require('node:child_process')
  const { server } = createFixture()
  let frontend
  server.listen(0, '127.0.0.1', () => {
    frontend = spawn(process.execPath, ['.output/server/index.mjs'], {
      cwd: path.join(root, 'frontend'), stdio: 'inherit',
      env: { ...process.env, NITRO_HOST: '127.0.0.1', NITRO_PORT: process.env.BLOG_PREVIEW_PORT || '13030', NUXT_BACKEND_BASE: `http://127.0.0.1:${server.address().port}` },
    })
    frontend.on('exit', code => { server.close(); process.exitCode = code || 0 })
  })
  for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => { frontend?.kill(signal); server.close() })
}
