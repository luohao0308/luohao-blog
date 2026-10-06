import { currentAccount, fetchMe, isLoggedIn, logout as authLogout, refreshSession, wechatLogin, bindWechat } from '../../utils/auth'
import type { Account } from '../../utils/types'
import { toMessage } from '../../utils/request'

Page({
  data: {
    mode: 'loading',
    account: null as Account | null,
    bindTicket: '',
    email: '',
    password: '',
    busy: false,
    error: '',
  },

  onShow() {
    this.restore()
  },

  async restore() {
    if (!isLoggedIn()) {
      // 静默刷新一次：15 分钟 access 过期但 refresh 还活着时免重登。
      const ok = await refreshSession()
      if (!ok) {
        this.setData({ mode: 'guest', account: null })
        return
      }
    }
    try {
      const account = await fetchMe()
      this.setData({ mode: 'signedin', account })
    } catch {
      this.setData({ mode: 'guest', account: null })
    }
  },

  async startWechatLogin() {
    if (this.data.busy) return
    this.setData({ busy: true, error: '' })
    try {
      const result = await wechatLogin()
      if (result.kind === 'ok') {
        this.setData({ mode: 'signedin', account: result.account, busy: false })
        return
      }
      this.setData({ mode: 'binding', bindTicket: result.ticket, busy: false })
    } catch (error) {
      this.setData({ busy: false, error: toMessage(error) })
    }
  },

  onEmailInput(event: WechatMiniprogram.Input) {
    this.setData({ email: event.detail.value })
  },

  onPasswordInput(event: WechatMiniprogram.Input) {
    this.setData({ password: event.detail.value })
  },

  async submitBind() {
    if (this.data.busy) return
    const { bindTicket, email, password } = this.data
    if (!email.trim() || !password) {
      this.setData({ error: '请输入已注册的邮箱和密码' })
      return
    }
    this.setData({ busy: true, error: '' })
    try {
      const account = await bindWechat(bindTicket, email.trim(), password)
      this.setData({ mode: 'signedin', account, busy: false, password: '' })
    } catch (error) {
      // 票据一次性：失败即作废，回登录页让用户重新 wx.login。
      this.setData({ mode: 'guest', busy: false, error: toMessage(error) })
    }
  },

  cancelBind() {
    this.setData({ mode: 'guest', bindTicket: '', password: '', error: '' })
  },

  async signOut() {
    await authLogout()
    this.setData({ mode: 'guest', account: null })
  },

  refreshMe() {
    if (currentAccount()) {
      this.restore()
    }
  },
})
