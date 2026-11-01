// Headers also cover SSR error pages and endpoints that do not render app.vue.
import { siteOrigin } from '../../app/utils/seo'

export default defineNitroPlugin((nitroApp) => {
  nitroApp.hooks.hook('error', (_error, context) => {
    if (context.event && !context.event.node.res.headersSent)
      setResponseHeader(context.event, 'X-Robots-Tag', 'noindex, nofollow')
  })
  // Nuxt errors may send directly, bypassing H3's beforeResponse hook.
  nitroApp.hooks.hook('request', (event) => {
    const path = getRequestURL(event).pathname
    if (/^\/(?:admin|login|register|settings|collections|preview|forgot-password|reset-password|verify-email|api|health|__nuxt_error)(?:\/|$)/.test(path)
      || !siteOrigin(useRuntimeConfig(event).public.siteUrl)) {
      setResponseHeader(event, 'X-Robots-Tag', 'noindex, nofollow')
    }
  })
  nitroApp.hooks.hook('render:response', (response) => {
    if ((response.statusCode ?? 200) >= 400) {
      response.headers ||= {}
      response.headers['x-robots-tag'] = 'noindex, nofollow'
    }
  })
  nitroApp.hooks.hook('beforeResponse', (event) => {
    const path = getRequestURL(event).pathname
    const privatePath = /^\/(?:admin|login|register|settings|collections|preview|forgot-password|reset-password|verify-email|api|health)(?:\/|$)/.test(path)
    if (privatePath || getResponseStatus(event) >= 400 || !siteOrigin(useRuntimeConfig(event).public.siteUrl)) {
      setResponseHeader(event, 'X-Robots-Tag', 'noindex, nofollow')
    }
  })
})
