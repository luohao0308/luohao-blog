# 博客域名与 HTTPS 切换

适用：现有 HTTP/IP 博客切换至 `https://makerhao.cn`。生产修改前完成准确版本授权、CI 和备份；不要把本指南存在当作线上已完成。

## 前置检查

- 核实服务器、部署目录、当前 Git SHA、前后端镜像 ID、Docker healthy、80/443 公网可达。
- 腾讯云 DNS 为 `@` 添加 A 记录到当前服务器 `193.112.128.245`；`www` 添加 CNAME 到 `makerhao.cn`。保存此前记录及 TTL，避免覆盖未知记录。
- 检查 A/AAAA/CNAME/CAA：没有真实 IPv6 服务时不添加 AAAA；如已有 CAA，核实允许 Caddy 使用的证书颁发机构。解析传播完成后再启用域名入口。
- 核实备案号：`渝ICP备2026025040号-1`；页脚应链接 `https://beian.miit.gov.cn/`。
- 保存 `.env.prod`、完整 `prod-conf`、Caddy 配置、当前镜像 ID 及准确旧 SHA 到受控备份目录（0600），按已有备份指南检查 DB/uploads 备份。不得输出配置中的凭据。

## S1 主站切换

保留生产现有密钥，只更新以下非秘密字段：

```dotenv
SITE_ADDRESS=makerhao.cn
CADDY_CONFIG=Caddyfile.domain
PUBLIC_SITE_URL=https://makerhao.cn
ICP_NUMBER=渝ICP备2026025040号-1
AUTH_SECURE_COOKIE=true
```

`CADDY_CONFIG` 是 `/etc/caddy` 内文件名；域名模式要求 SITE_ADDRESS 为裸主域名，不带协议/路径/www。compose 挂载整个 caddy 目录，保持 import 文件可达。所有页面及 `/api` 仍走 Nuxt，不能把 `/api` 直接转发 Kratos。

生产后端基础配置含原生布尔值 `auth.secure_cookie: false`，compose 通过 `KRATOS_AUTH_SECURE_COOKIE` 显式覆盖最终设置（只对该字段解析布尔值，非法值拒绝启动）。不要为该布尔字段使用字符串占位符。公众号私密配置仍按原模式保留。

根据已有发布 Runbook，在获准准确版本上构建/拉取前后端并更新 frontend/backend/caddy；只重建这些任务涉及的服务，不执行 compose down，不删除卷。Caddy 配置先用当前镜像 `caddy validate --config ... --adapter caddyfile` 验证，再重建容器。

域名 HTTPS 和 IP 是不同 cookie origin，现有 IP 登录态不能迁移；用户在域名入口重新登录。

### 验收

```bash
dig +short makerhao.cn A
dig +short www.makerhao.cn CNAME
curl -I http://makerhao.cn/
curl -I https://www.makerhao.cn/
curl -I https://makerhao.cn/
python3 scripts/e2e/domain_smoke.py
```

检查 HTTP 与 www 到主域名的重定向保留路径及 query。TLS 检查使用系统 CA，禁止 `curl -k` 冒充有效证书验收。脚本验证备案展示、canonical、公开 API、登录 noindex、匿名 logout Secure/HttpOnly/SameSite 与 BFF Path。使用隔离测试账号补充真实登录→刷新→退出，清理测试会话；不得输出 cookie/token。浏览器检查文章/图片、混合内容、页脚桌面与移动布局。内部 Kuma 的 `http://caddy` 探针仍可用。

本地 HTTP 冒烟配置使用 `SITE_ADDRESS=:80`、`CADDY_CONFIG=Caddyfile`、`AUTH_SECURE_COOKIE=false`、`PUBLIC_SITE_URL=http://localhost:3000`，ICP_NUMBER 可为空；不要把生产 Secure cookie 测试期关闭后遗漏恢复。

## S2 微信地址

S1 验收后修改小程序入口与合法域名校验；在微信控制台核实实际使用的网络 API，配置 `https://makerhao.cn` 的 request 及需要的资源域名（不带 `/api/v1`）。微信控制台的合法域名与 web-view 业务域名是独立设置，没有 web-view 的小程序无需新增业务域名。

将公众号实际生效 `official_account.site_url` 改为 `https://makerhao.cn`，保留 enabled/mode、凭据、封面 ID 和 journal。本站 origin 自动成为图片允许来源；外部 CDN 仅按实际需要加入 allowlist，不放宽为任意域名。旧 HTTP IP 图片需逐项核验兼容与公开可达性，不能盲目全量修改文章或重传微信草稿。

## S3 状态页和拨测

S1/S2 验收后为 status 建立解析和最小反代，只暴露 `/status/blog` 所需公开页面/静态资源/状态 API。验收拒绝管理、登录及管理 socket；不公开 Grafana/Prometheus。external-probe 改 HTTPS，并重新验证内部健康探针和数据层信号。

## 失败与回退

- DNS/TLS 失败时保持或恢复原入口，按备份恢复 DNS、Caddy、PUBLIC_SITE_URL 和 cookie 配置。回退 HTTP 时必须同时恢复 AUTH_SECURE_COOKIE=false，否则浏览器无法使用 refresh cookie。
- 恢复旧配置与准确旧镜像，保留 DB、uploads、证书、Kuma/Prometheus 等数据卷；不做数据库迁移/删除。
- 微信回退只恢复 API/合法域名/site_url，保留已有草稿和任务；不以切换域名为理由重新推送历史文章。
- 每个切片部署证据保存于受控平台或本机 ignored artifacts，包括准确 SHA、镜像 ID、探针结果、旧配置备份指针及未完成项。
