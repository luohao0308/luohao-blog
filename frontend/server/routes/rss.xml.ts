// RSS 2.0 feed of the latest published articles, rendered server-side from
// the same backend API the site reads. Summaries keep the feed light: full
// content stays on the site.
interface FeedArticle {
  slug: string
  title: string
  summary: string
  content_md: string
  tags?: string[]
  published_at?: string | { seconds?: string | number }
  created_at: string | { seconds?: string | number }
}

interface ArticleSet {
  articles?: FeedArticle[]
}

// toDate accepts both wire shapes the API emits for timestamps (RFC3339
// string or protobuf {seconds}).
function toDate(ts?: FeedArticle['published_at']): Date {
  if (typeof ts === 'string') {
    const d = new Date(ts)
    if (!Number.isNaN(d.getTime()))
      return d
  }
  else if (ts?.seconds) {
    return new Date(Number(ts.seconds) * 1000)
  }
  return new Date(0)
}

function escapeXml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll('\'', '&apos;')
}

export default defineEventHandler(async (event) => {
  const { backendBase } = useRuntimeConfig(event)
  let articles: FeedArticle[] = []
  try {
    const set = await $fetch<ArticleSet>(`${backendBase}/v1/articles/list`, {
      query: { page_size: 20, order_by: 'published_at desc,slug' },
    })
    articles = set.articles ?? []
  }
  catch {
    // Backend unavailable: emit a valid, empty feed so readers keep a 200
    // instead of surfacing an error page for the subscribed URL.
  }

  const origin = getRequestURL(event).origin
  const items = articles
    .map((article) => {
      const url = `${origin}/posts/${encodeURIComponent(article.slug)}`
      const lines = [
        '    <item>',
        `      <title>${escapeXml(article.title)}</title>`,
        `      <link>${url}</link>`,
        `      <guid isPermaLink="true">${url}</guid>`,
        `      <description>${escapeXml(article.summary || article.content_md.slice(0, 200))}</description>`,
        `      <pubDate>${toDate(article.published_at || article.created_at).toUTCString()}</pubDate>`,
      ]
      for (const tag of article.tags ?? [])
        lines.push(`      <category>${escapeXml(tag)}</category>`)
      lines.push('    </item>')
      return lines.join('\n')
    })
    .join('\n')
  const latest = articles[0]
  const updated = latest ? toDate(latest.published_at || latest.created_at).toUTCString() : new Date().toUTCString()

  setResponseHeader(event, 'content-type', 'application/rss+xml; charset=utf-8')
  return `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
  <channel>
    <title>luohao.blog</title>
    <link>${origin}</link>
    <description>全栈工程师 luohao 的技术记录：Go、AI Agent 与工程实践。</description>
    <language>zh-CN</language>
    <lastBuildDate>${updated}</lastBuildDate>
    <atom:link href="${origin}/rss.xml" rel="self" type="application/rss+xml"/>
${items}
  </channel>
</rss>
`
})
