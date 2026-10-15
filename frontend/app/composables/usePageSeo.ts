import { DEFAULT_SEO_DESCRIPTION, publicAssetPath, siteOrigin, siteURL } from '~/utils/seo'

export function usePageSeo(input: {
  title: MaybeRefOrGetter<string>
  description?: MaybeRefOrGetter<string | undefined>
  image?: MaybeRefOrGetter<string | undefined>
  type?: 'website' | 'article'
}) {
  const route = useRoute()
  const config = useRuntimeConfig()
  const origin = siteOrigin(config.public.siteUrl)
  const title = computed(() => toValue(input.title))
  const description = computed(() => toValue(input.description)?.trim() || DEFAULT_SEO_DESCRIPTION)
  const canonical = computed(() => siteURL(origin, route.path === '/' ? '/' : route.path))
  const image = computed(() => siteURL(origin, publicAssetPath(toValue(input.image))))

  useHead({ title })
  useSeoMeta({
    description,
    ogTitle: title,
    ogDescription: description,
    ogType: input.type ?? 'website',
    ogUrl: canonical,
    ogImage: image,
    twitterCard: 'summary_large_image',
    twitterTitle: title,
    twitterDescription: description,
    twitterImage: image,
  })
}
