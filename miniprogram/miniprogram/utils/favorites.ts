// 本地收藏与点赞去重：与网页版一致走本机存储（账号体系暂不承载收藏同步）。
// 收藏记录保存 slug + 标题快照，收藏页无需逐篇回源。

export interface FavoriteItem {
  slug: string
  title: string
  savedAt: number
}

const FAV_KEY = 'blog:favorites'
const LIKE_PREFIX = 'blog:liked:'

export function listFavorites(): FavoriteItem[] {
  try {
    return (wx.getStorageSync(FAV_KEY) as FavoriteItem[]) || []
  } catch {
    return []
  }
}

export function isFavorite(slug: string): boolean {
  return listFavorites().some((item) => item.slug === slug)
}

export function toggleFavorite(slug: string, title: string): boolean {
  const items = listFavorites()
  const index = items.findIndex((item) => item.slug === slug)
  if (index >= 0) {
    items.splice(index, 1)
    try {
      wx.setStorageSync(FAV_KEY, items)
    } catch { /* ignore */ }
    return false
  }
  items.unshift({ slug, title, savedAt: Date.now() })
  try {
    wx.setStorageSync(FAV_KEY, items)
  } catch { /* ignore */ }
  return true
}

export function removeFavorite(slug: string) {
  const items = listFavorites().filter((item) => item.slug !== slug)
  try {
    wx.setStorageSync(FAV_KEY, items)
  } catch { /* ignore */ }
}

// isLiked / markLiked 记录本机点赞状态，避免重复请求后端（后端本身也有 24h 去重）。
export function isLiked(slug: string): boolean {
  try {
    return Boolean(wx.getStorageSync(LIKE_PREFIX + slug))
  } catch {
    return false
  }
}

export function markLiked(slug: string) {
  try {
    wx.setStorageSync(LIKE_PREFIX + slug, true)
  } catch { /* ignore */ }
}
