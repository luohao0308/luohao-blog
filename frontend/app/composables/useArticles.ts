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

export interface Article {
  id: string
  slug: string
  title: string
  summary: string
  content_md: string
  content_html: string
  tags: string[]
  status: ArticleStatus
  published_at?: string
  created_at: string
  updated_at: string
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
  return useFetch<Article>(`/api/v1/articles/${slug}`, { key: `article:${slug}` })
}

// formatDate renders an RFC3339 timestamp as a plain local date.
export function formatDate(ts?: string): string {
  if (!ts) return ''
  return new Date(ts).toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
}
