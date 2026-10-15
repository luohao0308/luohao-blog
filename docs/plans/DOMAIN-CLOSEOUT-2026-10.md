# 备案后域名收尾

- 状态：`production_delivered_client_acceptance_pending`
- 日期：2026-10-13
- 关联：T-008、T-011、T-013、T-015
- 主域名：`makerhao.cn`
- 用户提供的备案号：`渝ICP备2026025040号-1`


## 当前交付状态（2026-10-16 对齐）

S1 主站 HTTPS/备案展示/Secure Cookie 与 S3 受限公开状态页已交付；S2 小程序 HTTPS 地址和合法域名校验代码、公众号 HTTPS 原文已交付，客户端完整真机验收仍待完成。当前生产与 main 为 `30e291dc2167229e6018bd3cd3e1a540a7607ea0`，以下立项证据与旧镜像标记保留其历史时点。

本轮只读核验主站200、`https://status.makerhao.cn` 最终进入 `/status/blog` 返回200，公开状态与 heartbeat API200，管理页面、其他状态页 slug 和 socket入口404。该检查不替代完整42项公开代理负向复验；最近一次 [external-probe](https://github.com/luohao0308/luohao-blog/actions/runs/38776227141) 于2026-10-16北京时间01:18启动并成功（源码main30e）；本轮未额外触发工作流。

SMTP/邮箱验证与密码找回已由后续 T-016 另行实现、上线并完成真实账号验收，不再作为未立项项。网站看门狗亦由 T-016 改用 Docker 容器健康状态。小程序自身备案/隐私指引、正式发布状态以及网站公安备案状态，仍需平台证据，不能从网站ICP备案号推定。

## 目标与边界

将现有博客公开入口切换为 https://makerhao.cn，补齐备案展示、HTTPS 会话、小程序及公众号地址、公开状态页和拨测。保持现有功能、数据库与公众号 draft 模式。

原域名切换轮不实现 SMTP 或发送邮件；后续账号邮箱功能已由 T-016 另行交付，首页邮件订阅不恢复。小程序正式提审、公众号公开发表/群发也不属于域名配置交付。

## 立项时只读证据（2026-10-13，历史）

- 当前工作树干净，但位于旧任务 codex/account-avatar-subscription（63dd394）；2026-10-13 git ls-remote 核实 main 为 39f8a524ae7920261dea4eb47cfa22784658b90f。实施从最新 main 隔离，避免混入旧任务。
- 1.1.1.1、8.8.8.8 查询 makerhao.cn A 均 NOERROR / 无 answer；1.1.1.1 查询 www、status 为 NXDOMAIN。此为查询时的观察，实施前重新验证 A/AAAA/CNAME/CAA。
- Caddy SITE_ADDRESS 默认 :80，全部反代 Nuxt BFF；禁止绕过 BFF 的 refresh cookie Path 重写。
- backend/internal/service/auth.go 签发和清除 refresh cookie 都未设置 Secure；需要明确生产配置及本地 HTTP 兼容，不盲信任客户端转发头。
- 小程序唯一入口仍是 HTTP IP，project.config.json urlCheck=false；实际使用的微信网络 API 决定需要的合法域名种类。
- 页脚无 ICP 展示，Nuxt 无公开站点 origin/canonical；外部 probe 仍用 HTTP IP。
- 公众号原文地址由 official_account.site_url 构造；生产实际值须在受控配置操作中核验，不能读取/打印密钥文件。
- Kuma 公开 /status/blog 已有规划，但生产可用路由/依赖应重新检查，公开代理必须只放行状态页必要路径，不公开管理、登录、socket 管理接口。

## 顺序切片

| 切片 | 结果与修改范围 | 依赖 | 验收 | 回退点 |
|---|---|---|---|---|
| S1 主站与安全会话 | DNS @ 指向已核验服务器，www 指向主站并统一跳转；Caddy 自动 HTTPS；backend Secure cookie 显式配置；Nuxt 公开 origin、canonical/OG 地址、ICP 页脚；compose/example 与部署指南同步 | 计划批准；实际 DNS/生产配置访问；备案号已提供 | Cookie 设置/清除回归与 HTTP 开发兼容；Go build/test/lint；pnpm lint/typecheck/build；Caddy validate；真实证书、HTTP/www 跳转、SSR 备案链接、API、登录/刷新/退出、图片及混合内容检查 | 保存 DNS 旧值、Caddy/生产配置与准确旧镜像；恢复旧入口及 cookie 配置；不删除证书卷或数据卷 |
| S2 微信地址闭环 | 小程序 API_BASE 改 HTTPS，开启合法域名校验，配置实际需要的 request/图片域名；公众号 site_url 切 HTTPS，核验图片来源允许列表 | S1 HTTPS 验收；微信控制台可用权限 | npm run typecheck + npm test；开启校验的 DevTools/真机阅读、登录、评论、图片；公众号 URL 构造/allowlist 回归与受控诊断，不盲重传已有草稿 | 回退代码/微信域名配置/site_url；保留草稿及任务 journal，禁止重新推送历史文章 |
| S3 监控与交付收尾 | status 子域解析和最小公开反代；外部 probe 改 HTTPS；核验 Kuma 内部探针在 HTTPS 切换后仍有效；更新 README/Runbook/TASKS 与部署证据 | S1、S2；Kuma 状态页实际路由核验 | 状态页和必要静态/API 请求成功，管理/登录/非允许 API 被拒；内部指标/健康探针正常，公网拨测成功；PR required CI 通过，部署准确 SHA 后公开浏览器回归 | 回退状态页代理及拨测配置，保留监控数据；主站入口保持已验收版本 |

每次最多一个切片 in_progress，验收后推进。实施时为每条关键行为先准备最小可执行检查；DNS/证书等外部状态以切换前后探针作证据。SEO robots/sitemap 如需新增动态枚举接口，先评估现有数据读取入口，不扩大 API 契约或暴露草稿/后台页面。

## 外部操作与授权

计划批准不修改持久 gitPolicy。push/PR/merge 仍需准确 ref/SHA 授权并通过 delivery_guard；生产更新在本地代码、验证和回退材料完成后绑定准确版本授权。DNS/微信控制台操作按批准的具体记录与域名范围执行；缺少登录/权限时保留可执行材料并报告实际阻塞。

## 发信域名准备（待另行立项）

DirectMail 域名验证、SPF/DKIM/DMARC 等记录必须由选定账号的实际控制台产生并核验，不能猜测 TXT 值。本轮不从备案通过推导邮件发送功能或凭据操作授权。

## 官方参考

- [Caddy 自动 HTTPS](https://caddyserver.com/docs/automatic-https)：域名与公网 80/443 可达性及自动跳转。
- [腾讯云备案号悬挂说明](https://cloud.tencent.com/document/product/243/61412)：准确备案号展示于页脚并链接 https://beian.miit.gov.cn/。

## 进度

- S1：completed。PR #90/#91；生产 Git ed8be10 与 main d206e0e 树一致，运行镜像775455a3（修复仅检查脚本/测试）。DNS/TLS、ICP/canonical、HTTP/www/IP 308、公开API/多页资源、真实注册/登录/刷新/退出/401、Secure/HttpOnly/Lax/Path、内部健康探针验收通过；任务临时账号与会话已精确清理。
- S2：代码与生产配置已交付，客户端验收待完成。PR #92（main a3c36bc / source02344ac）CI与独立复核通过；小程序HTTPS与默认域名校验已合并，微信后台request/downloadFile由用户确认保存。重新授权后准确源码检出成功，生产公众号origin为https://makerhao.cn，enabled/draft_mode保留，零同步任务，backend healthy、HTTPS smoke与管理员匿名401通过，看门狗逐字恢复。运行镜像仍为775455a3；未重传封面或重放历史任务。DevTools/真机验证因IDE服务端口关闭未完成。
- S3：已由 [PR #93](https://github.com/luohao0308/luohao-blog/pull/93) 合并并生产交付（merge `fade43abed5578236a340e7034eba0df63401aff`，当前生产包含该提交）。2026-10-16 本轮只读核验公开状态页/公开API200及管理、其他slug、socket入口404；不再将旧“status CNAME待配置”当作当前阻塞。此前完整42项正负检查保留原时点，最近外部拨测成功记录见本文件顶部，本轮没有重新触发。
- 首次验收脚本误将删除Cookie的wire Max-Age=0视为-1，自动回退后已修复，并通过真实Go→Nitro回归与重新授权部署。恢复点、消费记录和动态证据保存在本机ignored artifacts与受控服务器备份目录。
