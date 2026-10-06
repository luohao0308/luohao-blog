import { listFavorites, removeFavorite, type FavoriteItem } from '../../utils/favorites'

interface FavRow {
  slug: string
  title: string
  dateText: string
}

Page({
  data: {
    items: [] as FavRow[],
  },

  onShow() {
    this.refresh()
  },

  refresh() {
    const items = listFavorites().map((item: FavoriteItem) => ({
      slug: item.slug,
      title: item.title,
      dateText: new Date(item.savedAt).toISOString().slice(0, 10).replace(/-/g, '-'),
    }))
    this.setData({ items })
  },

  openPost(event: WechatMiniprogram.TouchEvent) {
    const slug = event.currentTarget.dataset.slug as string
    wx.navigateTo({ url: `/pages/post/post?slug=${encodeURIComponent(slug)}` })
  },

  removeOne(event: WechatMiniprogram.TouchEvent) {
    const slug = event.currentTarget.dataset.slug as string
    removeFavorite(slug)
    this.refresh()
    wx.showToast({ title: '已取消收藏', icon: 'none' })
  },
})
