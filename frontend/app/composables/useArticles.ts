// Typed access to the article API (proxied through /api by the server route).
// The Kratos HTTP codec emits the original proto field names (snake_case) and
// enums as numbers, matching the proto definitions in backend/api/blog/v1.

export type ArticleStatus = 0 | 1 | 2 | 3

export const ARTICLE_STATUS = {
  UNSPECIFIED: 0,
  DRAFT: 1,
  PUBLISHED: 2,
  DELETED: 3,
} as const

export const ARTICLE_STATUS_LABEL: Record<number, string> = {
  0: '未知',
  1: '草稿',
  2: '已发布',
  3: '已删除',
}

export type ArticleTimestamp = string | { seconds?: string | number, nanos?: number }

export interface Article {
  id: string
  version?: string
  slug: string
  title: string
  summary: string
  content_md: string
  content_html: string
  tags: string[]
  // Single curated category; absent when the article is uncategorized.
  category?: { slug: string, name: string } | null
  status: ArticleStatus
  published_at?: ArticleTimestamp
  created_at: ArticleTimestamp
  updated_at: ArticleTimestamp
  view_count?: number
  like_count?: number
  // Media paths are stored against the backend API origin and rendered via
  // assetUrl(). Missing fields keep older articles wire-compatible.
  cover_url?: string
  featured?: boolean
  reading_minutes?: number
}

export interface ArticleSet {
  articles: Article[]
  next_page_token?: string
  search_outcome?: number | string
}

export function useArticleList(opts?: { pageSize?: number, filter?: string, orderBy?: string }) {
  return useFetch<ArticleSet>('/api/v1/articles/list', {
    query: {
      page_size: opts?.pageSize ?? 20,
      filter: opts?.filter,
      order_by: opts?.orderBy ?? 'created_at desc',
    },
  })
}

export function useArticleBySlug(slug: string) {
  return useFetch<Article>(`/api/v1/articles/${encodeURIComponent(slug)}`, { key: `article:${slug}` })
}

// The directory preserves ordering and facets without sending Markdown or HTML.
function usePublishedArticleDirectory() {
  const requestFetch = useRequestFetch()
  return async () => {
    const articles: Article[] = []
    const seenTokens = new Set<string>()
    let pageToken = ''
    do {
      const page = await requestFetch<ArticleSet>('/api/v1/articles/list', {
        query: { page_size: 100, page_token: pageToken, order_by: 'published_at desc,slug', summary_only: true },
      })
      for (const article of page.articles ?? []) {
        if (article.status !== ARTICLE_STATUS.PUBLISHED) continue
        // Older backends may return full bodies during a rolling update.
        const { content_md, ...summary } = article
        articles.push({
          ...summary,
          summary: articleSummary(summary.summary, content_md),
          reading_minutes: summary.reading_minutes ?? readingMinutes(content_md),
          content_md: '',
          content_html: '',
        })
      }
      pageToken = page.next_page_token ?? ''
      if (pageToken && seenTokens.has(pageToken)) throw new Error('文章分页异常')
      seenTokens.add(pageToken)
    } while (pageToken)
    return articles
  }
}

export function usePublishedArticles() {
  return useAsyncData('published-articles', usePublishedArticleDirectory())
}

export function useHomepageArticles() {
  const fetchDirectory = usePublishedArticleDirectory()
  return useAsyncData('homepage-articles', async () => {
    const articles = await fetchDirectory()
    return {
      latest: articles.slice(0, 3),
      articleCount: articles.length,
      tagCount: new Set(articles.flatMap(article => article.tags ?? [])).size,
      latestUpdated: articles.reduce<ArticleTimestamp | undefined>((latest, article) =>
        articleDate(article.updated_at) > articleDate(latest) ? article.updated_at : latest, undefined),
    }
  })
}

export async function searchPublishedArticles(query: string, pageSize = 100): Promise<Article[]> {
  const requestFetch = useRequestFetch()
  const page = await requestFetch<ArticleSet>('/api/v1/search/articles', {
    query: { query, page_size: pageSize },
  })
  if (page.search_outcome === 2 || page.search_outcome === 'SEARCH_OUTCOME_UNAVAILABLE')
    throw createError({ statusCode: 503, message: '搜索服务暂不可用' })
  return (page.articles ?? []).filter(article => article.status === ARTICLE_STATUS.PUBLISHED)
}

// tagCounts aggregates how often each tag appears across articles, ordered by
// usage count desc then name, for the tag cloud and the tags index page.
export function tagCounts(articles: Article[]): Array<{ tag: string, count: number }> {
  const counts = new Map<string, number>()
  for (const article of articles) {
    for (const tag of article.tags ?? []) {
      counts.set(tag, (counts.get(tag) ?? 0) + 1)
    }
  }
  return [...counts.entries()]
    .map(([tag, count]) => ({ tag, count }))
    .sort((a, b) => b.count - a.count || a.tag.localeCompare(b.tag, 'zh-CN'))
}

// formatDate renders an RFC3339 timestamp as a plain local date.
export function articleDate(ts?: ArticleTimestamp): string {
  if (!ts) return ''
  const date = new Date(typeof ts === 'string' ? ts : Number(ts.seconds ?? 0) * 1000)
  return Number.isNaN(date.getTime()) ? '' : date.toISOString()
}

export function formatDate(ts?: ArticleTimestamp): string {
  const value = articleDate(ts)
  if (!value) return ''
  return new Date(value).toLocaleDateString('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
}
