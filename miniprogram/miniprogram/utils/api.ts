import { http, UnauthorizedError } from './request'
import { currentAccount, currentIdentityEpoch, clearSession } from './auth'
import type { Article, ArticleSet, CommentSet, MiniCommentRecord, MiniCommentRecords, WechatLoginReply, ChatReply } from './types'

// listArticles 按发布时间倒序取公开文章分页；与 Web 端 usePublishedArticles 同参。
// filter 为可选的列表过滤器，如 tag:"go"、category:"engineering"。
export function listArticles(pageToken: string, pageSize = 20, filter = ''): Promise<ArticleSet> {
  return http.get<ArticleSet>('/articles/list', {
    page_size: pageSize,
    page_token: pageToken || undefined,
    order_by: 'published_at desc',
    filter: filter || undefined,
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

// listComments 拉取文章已通过审核的公开评论。
export function listComments(slug: string, pageToken = ''): Promise<CommentSet> {
  return http.get<CommentSet>(`/mini/articles/${encodeURIComponent(slug)}/comments`, {
    page_size: 20,
    page_token: pageToken || undefined,
  })
}

function scopedMiniRequest<T>(request: Promise<T>, guard: () => boolean): Promise<T> {
  return request.catch(error => {
    // A definitive auth rejection without a refresh token also invalidates
    // local mini identity. Never clear a successor session on an old error.
    if (error instanceof UnauthorizedError && guard()) clearSession()
    throw error
  })
}

function freshWechatCode(): Promise<string> {
  return new Promise((resolve, reject) => wx.login({
    success: reply => reply.code ? resolve(reply.code) : reject(new Error('微信身份获取失败，请重试')),
    fail: () => reject(new Error('微信身份获取失败，请重试')),
  }))
}

export async function createComment(slug: string, content: string, requestId: string): Promise<MiniCommentRecord> {
  const owner = currentAccount()?.id
  const epoch = currentIdentityEpoch()
  const guard = () => Boolean(owner) && owner === currentAccount()?.id && epoch === currentIdentityEpoch()
  const code = await freshWechatCode()
  if (!guard()) throw new Error('登录账号已变化，请重新操作')
  return scopedMiniRequest(http.postTimeout<MiniCommentRecord>('/mini/comments', { article_slug: slug, content, wechat_code: code, client_request_id: requestId }, 25000, guard), guard)
}

export function listMyComments(slug: string, pageToken = ''): Promise<MiniCommentRecords> {
  const owner = currentAccount()?.id
  const epoch = currentIdentityEpoch()
  const guard = () => Boolean(owner) && owner === currentAccount()?.id && epoch === currentIdentityEpoch()
  return scopedMiniRequest(http.get<MiniCommentRecords>(`/mini/articles/${encodeURIComponent(slug)}/comments/mine`, { page_size: 20, page_token: pageToken || undefined }, guard), guard)
}

export async function recheckComment(id: string): Promise<MiniCommentRecord> {
  const owner = currentAccount()?.id
  const epoch = currentIdentityEpoch()
  const guard = () => Boolean(owner) && owner === currentAccount()?.id && epoch === currentIdentityEpoch()
  const code = await freshWechatCode()
  if (!guard()) throw new Error('登录账号已变化，请重新操作')
  return scopedMiniRequest(http.postTimeout<MiniCommentRecord>(`/mini/comments/${encodeURIComponent(id)}/recheck`, { wechat_code: code }, 25000, guard), guard)
}

// searchArticles 走 ES 混合搜索（关键词 + 语义）。
export function searchArticles(query: string, pageToken = ''): Promise<ArticleSet> {
  return http.get<ArticleSet>('/search/articles', {
    query,
    page_size: 20,
    page_token: pageToken || undefined,
  })
}

// listCategories 分类列表（生产暂无数据，建了分类即自动出现）。
export function listCategories(): Promise<{ categories: Array<{ slug: string, name: string }> }> {
  return http.get('/categories/list')
}

// chatAsk RAG 问答：从已发布文章里检索作答；LLM 生成可达数十秒，用长超时。
export function chatAsk(query: string): Promise<ChatReply> {
  return http.postTimeout('/chat', { query }, 60_000)
}
