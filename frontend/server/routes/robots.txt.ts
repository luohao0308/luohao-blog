import { siteOrigin, siteURL } from '../../app/utils/seo'

export default defineEventHandler((event) => {
  const { public: publicConfig } = useRuntimeConfig(event)
  const origin = siteOrigin(publicConfig.siteUrl)
  setResponseHeader(event, 'Content-Type', 'text/plain; charset=utf-8')
  return [
    'User-agent: *',
    'Allow: /',
    'Disallow: /admin/',
    'Disallow: /login',
    'Disallow: /register',
    'Disallow: /settings',
    'Disallow: /collections',
    'Disallow: /forgot-password',
    'Disallow: /reset-password',
    'Disallow: /verify-email',
    `Sitemap: ${siteURL(origin, '/sitemap.xml')}`,
    '',
  ].join('\n')
})
