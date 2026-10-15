import type { ArticleTimestamp } from '~/composables/useArticles'

export interface Project {
  id?: string
  slug: string
  name: string
  summary: string
  description: string
  stack: string[]
  role: string
  repo_url?: string
  live_url?: string
  emoji: string
  year: string
  featured: boolean
  cover_url?: string
  screenshots?: string[]
  problem?: string
  approach?: string
  outcome?: string
  published: boolean
  sort: number
  created_at?: ArticleTimestamp
  updated_at?: ArticleTimestamp
}

export interface ProjectSet {
  projects: Project[]
  next_page_token?: string
}

export interface AboutSkill {
  group: string
  items: string[]
}

export interface AboutTimeline {
  year: string
  text: string
}

export interface AboutContact {
  label: string
  href: string
  text: string
}

export interface AboutLink {
  label: string
  href: string
}

export interface AboutProfile {
  id?: string
  name: string
  headline: string
  intro: string
  profile_image?: string
  current_focus: string
  skills: AboutSkill[]
  timeline: AboutTimeline[]
  contacts: AboutContact[]
  representative_links: AboutLink[]
  updated_at?: ArticleTimestamp
}

export async function usePublicProjects() {
  const requestFetch = useRequestFetch()
  const nuxtApp = useNuxtApp()
  const result = await useAsyncData('public-projects', async () => {
    const projects: Project[] = []
    const seenTokens = new Set<string>()
    let pageToken = ''
    do {
      const page = await requestFetch<ProjectSet>('/api/v1/projects/list', {
        query: { page_size: 100, page_token: pageToken || undefined, published: true },
        cache: 'no-store',
      })
      projects.push(...(page.projects ?? []).filter(project => project.published))
      pageToken = page.next_page_token ?? ''
      if (pageToken && seenTokens.has(pageToken)) throw new Error('项目分页异常')
      seenTokens.add(pageToken)
    } while (pageToken)
    return projects
  }, {
    // Reuse the SSR payload during hydration, then revalidate later SPA
    // navigation so publish/unpublish changes cannot leave stale lists.
    getCachedData: (key, nuxtApp) => nuxtApp.isHydrating ? nuxtApp.payload.data[key] : undefined,
  })
  if (import.meta.client && !nuxtApp.isHydrating) await result.refresh()
  return result
}

export async function useProjectBySlug(slug: string) {
  const nuxtApp = useNuxtApp()
  const result = await useFetch<Project>(`/api/v1/projects/${encodeURIComponent(slug)}`, {
    key: `project:${slug}`,
    cache: 'no-store',
    // A project can be unpublished from the admin UI during the same SPA
    // session. Revalidate detail navigation so a stale public cache cannot
    // keep a newly hidden project visible.
    getCachedData: (key, nuxtApp) => nuxtApp.isHydrating ? nuxtApp.payload.data[key] : undefined,
  })
  if (import.meta.client && !nuxtApp.isHydrating) await result.refresh()
  return result
}

export async function useAboutProfile() {
  const nuxtApp = useNuxtApp()
  const result = await useFetch<AboutProfile>('/api/v1/about', {
    key: 'about-profile',
    cache: 'no-store',
    getCachedData: (key, nuxtApp) => nuxtApp.isHydrating ? nuxtApp.payload.data[key] : undefined,
  })
  if (import.meta.client && !nuxtApp.isHydrating) await result.refresh()
  return result
}

export function useAdminPortfolio() {
  const { authFetch } = useAuth()

  async function listProjects(published?: boolean): Promise<Project[]> {
    const projects: Project[] = []
    const seenTokens = new Set<string>()
    let pageToken = ''
    do {
      const page = await authFetch<ProjectSet>('/api/v1/projects/list', {
        query: { page_size: 100, page_token: pageToken || undefined, published },
      })
      projects.push(...(page.projects ?? []))
      pageToken = page.next_page_token ?? ''
      if (pageToken && seenTokens.has(pageToken)) throw new Error('项目分页异常')
      seenTokens.add(pageToken)
    } while (pageToken)
    return projects
  }

  const getProject = (slug: string) => authFetch<Project>(`/api/v1/projects/${encodeURIComponent(slug)}`)
  const createProject = (project: Project) => authFetch<Project>('/api/v1/admin/projects', { method: 'POST', body: project })
  const updateProject = (project: Project) => authFetch<Project>(`/api/v1/admin/projects/${encodeURIComponent(project.slug)}`, {
    method: 'PUT',
    query: { update_mask: 'name,summary,description,stack,role,repo_url,live_url,emoji,year,featured,cover_url,screenshots,problem,approach,outcome,published,sort' },
    body: project,
  })
  const unpublishProject = (slug: string) => authFetch(`/api/v1/admin/projects/${encodeURIComponent(slug)}`, { method: 'DELETE' })
  const getAbout = () => authFetch<AboutProfile>('/api/v1/about')
  const updateAbout = (about: AboutProfile) => authFetch<AboutProfile>('/api/v1/admin/about', {
    method: 'PUT',
    // GET metadata is read-only; its RFC3339 updated_at is not an editable
    // field and cannot be decoded by the message-field request binding.
    body: {
      name: about.name,
      headline: about.headline,
      intro: about.intro,
      profile_image: about.profile_image,
      current_focus: about.current_focus,
      skills: about.skills,
      timeline: about.timeline,
      contacts: about.contacts,
      representative_links: about.representative_links,
    },
  })

  return { listProjects, getProject, createProject, updateProject, unpublishProject, getAbout, updateAbout }
}

export function safePortfolioHref(value?: string | null, allowMailto = false): string {
  const href = value?.trim() ?? ''
  if (!href || /[\r\n]/.test(href)) return ''
  try {
    const url = new URL(href)
    if (url.protocol === 'http:' || url.protocol === 'https:') return href
    if (allowMailto && url.protocol === 'mailto:' && /^[^\s@]+@[^\s@]+$/.test(url.pathname)) return href
  }
  catch {
    return ''
  }
  return ''
}
