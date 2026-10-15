import { siteOrigin, siteURL, sitemapLastmod, xmlEscape } from '../../app/utils/seo'

interface SitemapArticle {
  slug: string
  status: number
  tags?: string[]
  category?: { slug?: string } | null
  updated_at?: string | { seconds?: string | number }
}

interface SitemapProject {
  slug: string
  published: boolean
  updated_at?: string | { seconds?: string | number }
}

const STATIC_PATHS = ['/', '/posts', '/projects', '/about', '/archives', '/tags', '/categories', '/ranking']
const MAX_URLS = 50_000
const MAX_XML_BYTES = 50 * 1024 * 1024
const MAX_PAGES_PER_RESOURCE = 500

async function collectPages<T>(
  endpoint: string,
  itemKey: 'articles' | 'projects',
  query: Record<string, string | number | boolean | undefined>,
): Promise<T[]> {
  const items: T[] = []
  const seenTokens = new Set<string>()
  let pageToken = ''
  let pageCount = 0
  do {
    pageCount++
    if (pageCount > MAX_PAGES_PER_RESOURCE) throw new Error(`${itemKey} page limit exceeded`)
    const page = await $fetch<Record<string, unknown>>(endpoint, {
      query: { ...query, page_token: pageToken || undefined },
      cache: 'no-store',
      retry: 0,
      timeout: 10_000,
    })
    const pageItems = page[itemKey]
    if (!Array.isArray(pageItems)) throw new Error(`invalid ${itemKey} response`)
    items.push(...pageItems as T[])
    if (items.length > MAX_URLS) throw new Error(`${itemKey} record limit exceeded`)
    const next = typeof page.next_page_token === 'string' ? page.next_page_token : ''
    if (next && pageItems.length === 0) throw new Error(`empty ${itemKey} page returned a continuation token`)
    if (next && seenTokens.has(next)) throw new Error(`repeated ${itemKey} page token`)
    if (next) seenTokens.add(next)
    pageToken = next
  } while (pageToken)
  return items
}

export default defineEventHandler(async (event) => {
  setResponseHeader(event, 'Cache-Control', 'no-store')
  const config = useRuntimeConfig(event)
  const origin = siteOrigin(config.public.siteUrl)
  try {
    const [rawArticles, rawProjects] = await Promise.all([
      collectPages<SitemapArticle>(`${config.backendBase}/v1/articles/list`, 'articles', {
        page_size: 100,
        filter: 'status:"PUBLISHED"',
        order_by: 'updated_at desc,slug',
      }),
      collectPages<SitemapProject>(`${config.backendBase}/v1/projects/list`, 'projects', {
        page_size: 100,
        published: true,
      }),
    ])

    const articles = rawArticles.filter(article => article.status === 2)
    const projects = rawProjects.filter(project => project.published)
    const entries = new Map<string, string | undefined>()
    const add = (path: string, lastmod?: string) => {
      const current = entries.get(path)
      if (!entries.has(path) || (lastmod && (!current || lastmod > current))) entries.set(path, lastmod)
      if (entries.size > MAX_URLS) throw new Error('sitemap URL limit exceeded')
    }

    for (const path of STATIC_PATHS) add(path)
    for (const article of articles) {
      const lastmod = sitemapLastmod(article.updated_at)
      add(`/posts/${encodeURIComponent(article.slug)}`, lastmod)
      for (const tag of article.tags ?? []) add(`/tags/${encodeURIComponent(tag)}`, lastmod)
      if (article.category?.slug) add(`/categories/${encodeURIComponent(article.category.slug)}`, lastmod)
    }
    for (const project of projects) add(`/projects/${encodeURIComponent(project.slug)}`, sitemapLastmod(project.updated_at))

    const body = [...entries.entries()].map(([path, lastmod]) => [
      '  <url>',
      `    <loc>${xmlEscape(siteURL(origin, path))}</loc>`,
      lastmod ? `    <lastmod>${xmlEscape(lastmod)}</lastmod>` : '',
      '  </url>',
    ].filter(Boolean).join('\n')).join('\n')
    const xml = `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${body}\n</urlset>\n`
    if (Buffer.byteLength(xml) > MAX_XML_BYTES) throw new Error('sitemap size limit exceeded')
    setResponseHeader(event, 'Content-Type', 'application/xml; charset=utf-8')
    return xml
  }
  catch (error) {
    throw createError({ statusCode: 503, statusMessage: 'Sitemap temporarily unavailable', cause: error })
  }
})
