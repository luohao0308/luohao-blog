// nuxtjs-naive-ui 1.x collects css-render output in ssrContext.styles.
// Nuxt 4 renders styles through Unhead instead of consuming that legacy field.
// Run after the module's collector and preserve cssr-id for client hydration.
export default defineNuxtPlugin({
  name: 'naive-ssr-head',
  enforce: 'post',
  setup(nuxtApp) {
    nuxtApp.hook('app:rendered', () => {
      const context = nuxtApp.ssrContext
      if (!context || typeof context.styles !== 'string') return

      const styles = Array.from(
        context.styles.matchAll(/<style cssr-id="([^"]+)">([\s\S]*?)<\/style>/g),
        ([, id, css]) => ({ 'cssr-id': id, innerHTML: css }),
      )
      if (styles.length) context.head.push({ style: styles })
    })
  },
})
