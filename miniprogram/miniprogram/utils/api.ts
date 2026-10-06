import { http } from './request'
import type { Article, ArticleSet } from './types'

// listArticles 按发布时间倒序取公开文章分页；与 Web 端 usePublishedArticles 同参。
export function listArticles(pageToken: string, pageSize = 20): Promise<ArticleSet> {
  return http.get<ArticleSet>('/articles/list', {
    page_size: pageSize,
    page_token: pageToken || undefined,
    order_by: 'published_at desc',
  })
}

export function getArticle(slug: string): Promise<Article> {
  return http.get<Article>(`/articles/${encodeURIComponent(slug)}`)
}

// reportArticleViewed 上报阅读量；服务端做 24h 去重，失败可静默。
export function reportArticleViewed(slug: string): Promise<void> {
  return http.post<void>(`/articles/${encodeURIComponent(slug)}/view`, {})
}
