import { chatAsk } from '../../utils/api'
import { toMessage } from '../../utils/request'

interface Citation {
  slug: string
  title: string
}

interface ChatMessage {
  role: 'user' | 'assistant'
  text: string
  citations: Citation[]
  error?: boolean
}

// 开场引导：让访客知道可以问什么。
const GREETING: ChatMessage = {
  role: 'assistant',
  text: '我是这个博客的 AI 助手，读过站内所有已发布文章。问我任何和文章内容相关的问题，比如「写过哪些关于 Ent 的内容？」',
  citations: [],
}

Page({
  data: {
    messages: [GREETING],
    input: '',
    busy: false,
  },

  onInput(event: WechatMiniprogram.Input) {
    this.setData({ input: event.detail.value })
  },

  async send() {
    if (this.data.busy) return
    const question = this.data.input.trim()
    if (!question) return
    const messages = this.data.messages.concat([{ role: 'user' as const, text: question, citations: [] }])
    this.setData({ messages, input: '', busy: true })
    try {
      const reply = await chatAsk(question)
      this.setData({
        messages: (this.data.messages as ChatMessage[]).concat([{
          role: 'assistant',
          text: reply.answer || '（没有生成内容）',
          citations: (reply.citations ?? []).map((c) => ({ slug: c.slug, title: c.title })),
        }]),
        busy: false,
      })
      this.scrollBottom()
    } catch (error) {
      this.setData({
        messages: (this.data.messages as ChatMessage[]).concat([{
          role: 'assistant',
          text: toMessage(error),
          citations: [],
          error: true,
        }]),
        busy: false,
      })
      this.scrollBottom()
    }
  },

  scrollBottom() {
    wx.pageScrollTo({ scrollTop: 999999, duration: 200 })
  },

  openCitation(event: WechatMiniprogram.TouchEvent) {
    const slug = event.currentTarget.dataset.slug as string
    wx.navigateTo({ url: `/pages/post/post?slug=${encodeURIComponent(slug)}` })
  },
})
