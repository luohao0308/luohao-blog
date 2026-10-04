// Typed access to the category API (proxied through /api by the server
// route). Categories are the admin-curated single-level taxonomy; an article
// carries at most one of them and the wire format follows the snake_case
// proto fields defined in backend/api/blog/v1/category.proto.

import type { ArticleTimestamp } from '~/composables/useArticles'

export interface Category {
  id: string
  slug: string
  name: string
  sort: number
  article_count?: number
  created_at?: ArticleTimestamp
  updated_at?: ArticleTimestamp
}

export interface CategorySet {
  categories: Category[]
  next_page_token?: string
}

// useAdminCategories loads the full category list for management pages. The
// list endpoint is public, but management pages already hold an admin
// session and reuse authFetch so one loading path serves both.
export function useAdminCategories() {
  const { authFetch } = useAuth()
  const categories = ref<Category[]>([])
  const loading = ref(false)

  async function load() {
    loading.value = true
    try {
      const set = await authFetch<CategorySet>('/api/v1/categories/list', {
        query: { page_size: 100 },
      })
      categories.value = set.categories ?? []
    }
    finally {
      loading.value = false
    }
  }

  return { categories, loading, load }
}

// usePublicCategories loads the category list for the public site
// (navigation and aggregation pages). The endpoint is open; useAsyncData
// keeps it SSR-safe and deduplicated across pages.
export function usePublicCategories() {
  return useAsyncData('public-categories', async () => {
    const set = await $fetch<CategorySet>('/api/v1/categories/list', {
      query: { page_size: 100 },
    })
    return set.categories ?? []
  })
}
