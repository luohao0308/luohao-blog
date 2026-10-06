import { http } from './request'
import type { Article, ArticleSet, CommentSet, WechatLoginReply } from './types'

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

// wechatLogin 用 wx.login code 登录：绑定账号返回 login，新 openid 返回票据。
export function wechatLogin(code: string): Promise<WechatLoginReply> {
  return http.post<WechatLoginReply>('/auth/wechat', { code })
}

// bindWechat 用票据把 openid 绑到已有账号，返回标准登录响应。
export function bindWechat(ticket: string, email: string, password: string): Promise<import('./types').LoginReply> {
  return http.post('/auth/wechat/bind', { binding_ticket: ticket, email, password })
}

// likeArticle 点赞；服务端按客户端 24h 去重，重复调用不涨数。
export function likeArticle(slug: string): Promise<void> {
  return http.post<void>(`/articles/${encodeURIComponent(slug)}/like`, {})
}

// listComments 拉取文章评论（公开；登录态下会附带自己的待审评论）。
export function listComments(slug: string, pageToken = ''): Promise<CommentSet> {
  return http.get<CommentSet>(`/articles/${encodeURIComponent(slug)}/comments`, {
    page_size: 20,
    page_token: pageToken || undefined,
  })
}

// createComment 发表评论（需登录态；先审后显）。
export function createComment(slug: string, content: string): Promise<void> {
  return http.post<void>('/comments', { article_slug: slug, content })
}
