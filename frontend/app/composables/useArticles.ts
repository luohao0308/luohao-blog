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
}

export interface ArticleSet {
  articles: Article[]
  next_page_token?: string
}

export function useArticleList(opts?: { pageSize?: number }) {
  return useFetch<ArticleSet>('/api/v1/articles/list', {
    query: {
      page_size: opts?.pageSize ?? 20,
      order_by: 'created_at desc',
    },
  })
}

export function useArticleBySlug(slug: string) {
  return useFetch<Article>(`/api/v1/articles/${encodeURIComponent(slug)}`, { key: `article:${slug}` })
}

// Tags and adjacent articles need the full collection, not just its first page.
export function usePublishedArticles() {
  const requestFetch = useRequestFetch()
  return useAsyncData('published-articles', async () => {
    const articles: Article[] = []
    const seenTokens = new Set<string>()
    let pageToken = ''
    do {
      const page = await requestFetch<ArticleSet>('/api/v1/articles/list', {
        query: { page_size: 100, page_token: pageToken, order_by: 'published_at desc,slug' },
      })
      articles.push(...(page.articles ?? []))
      pageToken = page.next_page_token ?? ''
      if (pageToken && seenTokens.has(pageToken)) throw new Error('文章分页异常')
      seenTokens.add(pageToken)
    } while (pageToken)
    return articles.filter(article => article.status === ARTICLE_STATUS.PUBLISHED)
  })
}

export async function searchPublishedArticles(query: string, pageSize = 100): Promise<Article[]> {
  const requestFetch = useRequestFetch()
  const page = await requestFetch<ArticleSet>('/api/v1/search/articles', {
    query: { query, page_size: pageSize },
  })
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
