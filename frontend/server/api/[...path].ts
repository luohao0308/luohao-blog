// BFF proxy: /api/** -> backend Kratos HTTP server. Same path convention as
// the production Caddy rule (deploy/caddy/Caddyfile), so dev, preview and
// prod share one API surface and no CORS handling is needed anywhere.
export default defineEventHandler((event) => {
  const { backendBase } = useRuntimeConfig(event)
  // event.path = "/api/v1/articles/..."; the backend serves "/v1/...".
  const target = `${backendBase}${event.path.slice(4)}`
  // The backend scopes the refresh cookie to Path=/v1/auth, which never
  // matches the /api-prefixed browser requests, so rewrite it to the proxied
  // path; requests to /api/v1/auth/* then carry it and the BFF forwards the
  // Cookie header verbatim.
  return proxyRequest(event, target, {
    cookiePathRewrite: { '/v1/auth': '/api/v1/auth' },
  })
})
