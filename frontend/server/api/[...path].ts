// BFF proxy: /api/** -> backend Kratos HTTP server. Same path convention as
// the production Caddy rule (deploy/caddy/Caddyfile), so dev, preview and
// prod share one API surface and no CORS handling is needed anywhere.
export default defineEventHandler((event) => {
  const { backendBase } = useRuntimeConfig(event)
  // event.path = "/api/v1/articles/..."; the backend serves "/v1/...".
  const target = `${backendBase}${event.path.slice(4)}`
  return proxyRequest(event, target)
})
