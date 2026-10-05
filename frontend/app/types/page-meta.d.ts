// Custom page meta consumed by layouts/default.vue: `wide` opts the page into
// the wider container used by the two-column homepage.
export {}

declare module 'nuxt/app' {
  interface PageMeta {
    wide?: boolean
  }
}
