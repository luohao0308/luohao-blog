# SEO 与分享预览

- 日期：2026-11-01；用户已批准，L1 单轮前端交付；状态 completed（本地完成，未发布）。
- 目标：SSR 摘要、OG/Twitter 分享图、canonical、BlogPosting、robots 和 sitemap；后台/草稿/预览不进入公开索引。
- 现状：当前旧分支缺少 SEO；本地缓存 origin/main 已有基础 helper、composable 和爬虫路由。复用可用实现，适配当前静态作品集，不引入主分支其他 API、视觉或账号功能。
- 配置：沿用 NUXT_PUBLIC_SITE_URL；未配置/无效时禁用索引与 sitemap，绝不使用请求 Host 推导公开域。2026-11-01 已通过只读 HTTPS 请求确认 makerhao.cn 返回 200；不据此修改生产配置。
- 范围：前端 SEO helper、页面元信息、两条 Nitro 路由、专项测试与文档；无新依赖、数据库变更或部署。
- 最小验收：隔离 mock 的 121 篇分页文章、草稿过滤、XML/JSON 转义、标题与摘要和分享图、SPA 切换清理、私有页 noindex、无配置和上游失败、Host 污染；前端 lint/typecheck/build 与既有阅读回归。
- 回退：仅撤销本轮增量；不覆盖前两轮与原工作树改动。不包含 push/PR/生产部署。
- 治理边界：workflow_check 只读检查 warning；manifest 未启用技术 capability，未自动启用专项 profile，不声称 API/部署专项审查。

## 实施依据

- 复用缓存主分支中的 SEO helper、usePageSeo 与 sitemap/robots 路由设计；项目详情仍按本工作区静态数据生成，不调用尚未接入的项目 API。
- [Nuxt SEO 元信息](https://nuxt.com/docs/4.x/getting-started/seo-meta)、[Google Article 结构化数据](https://developers.google.com/search/docs/appearance/structured-data/article)、[sitemap 格式与限制](https://developers.google.com/search/docs/crawling-indexing/sitemaps/build-sitemap)。结构化数据不承诺排名或富结果展示。
- sitemap 不缓存、不携带请求者会话，最多 500 页、50,000 URL 和 50 MiB；分页循环/空页续页/上游失败返回 503，不返回残缺 200。
- SEO 配置缺失时禁止索引；私有路径/API/健康检查与错误响应也通过 X-Robots-Tag 标记。robots/noindex 是抓取与索引指令，不替代原有 API 鉴权。

## 故障路径补强

- 公开文章路由再核验 published 状态，上游误返草稿也返回 404，避免正文与元信息泄露。
- 错误页经过 Nuxt 内部错误渲染；用 request/render:response/beforeResponse 覆盖不同响应路径，错误与预览页面禁止索引。
- 非字符串分页 token 按上游格式错误处理，返回 503，不把异常 token 当作分页终点。

## 最终验收

- `frontend/`：`pnpm lint`、`pnpm typecheck`、`pnpm build` 全部通过；日志 `output/seo-preview/{lint,typecheck,build}.log`。
- `scripts/e2e/seo_preview_e2e.cjs` 全项 PASS（exit 0）；证据 `output/seo-preview/results.txt`。覆盖 121 篇分页、静态项目、公开筛选、XML/JSON 安全、元信息 SSR/SPA、封面、Host 污染、不转发用户会话、错误/noindex 与未配置/无效 origin。
- 原有搜索专项与阅读 1440/390px 明暗四组回归通过，证据 `output/seo-preview/search-regression.txt`、`reading-regression.txt`。独立审阅修复非法分页 token 被静默截断的问题。
- Nuxt 某些错误直接发送响应，已用 error hook 补齐 X-Robots-Tag；最终专项覆盖普通私有页、预览及草稿错误响应。
- 交付边界：所有原有改动保留；未提交、push、创建 PR 或部署，未改生产配置。真正启用抓取需在交付环境配置已核实的 NUXT_PUBLIC_SITE_URL，外部搜索引擎抓取和社交平台缓存未验证。

## 域名配置追补

- 用户要求直接填入正式地址；生产 Compose 默认传入 `NUXT_PUBLIC_SITE_URL=https://makerhao.cn`，允许环境变量覆盖，示例环境文件同步。
- 开发环境继续留空，避免本地页面意外索引；不改服务器环境文件或重建生产容器。
- 验证：用隔离占位变量和 `/dev/null` 环境文件运行 Compose config，默认域名与外部变量覆盖两种结果均通过；`git diff --check` 通过。未读取真实环境文件。
