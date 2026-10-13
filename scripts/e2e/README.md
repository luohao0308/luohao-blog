# E2E 自动化验证（Python + Playwright）

对本地生产栈（`deploy/compose.prod.yml`，caddy :8080）做黑盒 GUI 级验证的黑盒套件。

## 运行

```bash
# 一次性准备（PEP 668 下不要直接 pip install 到系统 Python）
python3 -m venv /tmp/pw-venv
/tmp/pw-venv/bin/pip install playwright
/tmp/pw-venv/bin/playwright install chromium

# 全量跑（需本地生产栈在 :8080 运行）
/tmp/pw-venv/bin/python scripts/e2e/blog_e2e.py

# 子集重跑（前缀匹配）
/tmp/pw-venv/bin/python scripts/e2e/blog_e2e.py t14 t15
```

- 默认目标 `http://127.0.0.1:8080`，可用 `BLOG_E2E_BASE_URL` 覆盖。
- 管理员凭据默认用本地 seed 示例账号（`author@example.com` / `longenough1`），
  可用 `BLOG_ADMIN_EMAIL` / `BLOG_ADMIN_PASSWORD` 覆盖。**不要把真实生产密码写进本目录或提交。**
- 截图输出到 `gui-test-screenshots/e2e-py/`。
- 用例自清理：创建的文章/评论在用例内删除；连续多轮运行可能触发登录限流
  （10 次/5min/IP），可清 Redis `blog:ratelimit:*` 或等 5 分钟。
- 退出码：存在「意外失败」时为 1；仅「已知缺陷复现」（known_issue 标注）不算失败。


## 评论专项本地测试

`comments_e2e.cjs` 验证分页、普通评论即时公开、待审预览、Unicode 长度、管理员翻页失败后重试和审核通过。它只允许 localhost/127.0.0.1，使用独立内存 SQLite + miniredis 测试服务，不依赖 Docker 或生产配置。

## 头像与 RSS 移除回归

`account_avatar_e2e.cjs` 对本地前端使用模拟账号 API，检查头像单次编码、编码失败回退、上传与刷新回显、非法格式、服务端拒绝、后台设置入口、移动端溢出，以及首页/页脚/发现标签和路由中的 RSS 移除。此用例不创建真实账号、不写入真实头像，不替代后端存储测试。

先在 `frontend/` 执行 `pnpm build`，再执行 `NITRO_HOST=127.0.0.1 NITRO_PORT=13000 node .output/server/index.mjs`。使用已有 Playwright 环境运行：

```bash
NODE_PATH=<playwright所在的node_modules目录> BLOG_E2E_BASE_URL=http://127.0.0.1:13000 node scripts/e2e/account_avatar_e2e.cjs
```

如需使用本机 Chrome，通过 `BLOG_E2E_CHROMIUM` 指定可执行文件路径。不要同时在同一个前端目录跑 dev 和 build，以免 Nuxt 缓存重建影响浏览器测试。

## 评论 fixture 启动步骤

1. 后端目录启动 fixture（保持这个终端运行；URL 写入文件，不包含凭据）：

   ```bash
   BLOG_COMMENT_BROWSER_FIXTURE=/tmp/blog-comment-fixture.url go test ./internal/data -run TestCommentHTTPModerationLifecycle -count=1 -v
   ```

2. 前端目录构建并启动本地生产产物（fixture 启动后才能读取 URL）：

   ```bash
   pnpm build
   NUXT_BACKEND_BASE="$(cat /tmp/blog-comment-fixture.url)" NITRO_HOST=127.0.0.1 NITRO_PORT=13000 node .output/server/index.mjs
   ```

3. 根目录运行专项脚本，复用已安装的 Playwright（不增加项目依赖）：

   ```bash
   NODE_PATH=<已有 Playwright 包目录> BLOG_E2E_BASE_URL=http://127.0.0.1:13000 node scripts/e2e/comments_e2e.cjs
   ```

   如已有浏览器版本与包版本不同，可设置 `BLOG_E2E_CHROMIUM=<现有 chromium 可执行文件>`；截图路径由 `BLOG_COMMENT_SCREENSHOT` 指定。fixture 账号仅为本地测试示例 `reader@example.com` / `author@example.com`，密码同为 `longenough1`。

4. 完成后停止本任务的前端服务，并通知 fixture 退出：

   ```bash
   curl -X POST "$(cat /tmp/blog-comment-fixture.url)/fixture/stop"
   ```

fixture 最长运行十分钟，然后自动结束；普通 `go test ./...` 不启动长期服务，只执行真实 HTTP 契约回归。生产 MySQL 分页精度差异另由相同时间戳分页回归覆盖，小程序测试不替代 DevTools/真机点验。

## 公众号同步本地验收

`wechat_sync_e2e.cjs` 对真实本地 HTTP 鉴权、SQLite outbox 和模拟微信 API 验证后台同步状态、同版本幂等入队、刷新回读、加载失败恢复、首页关注入口、移动端以及匿名登录守卫。无真实公众号发布/群发。

1. 在 `backend/` 启动独立 fixture：

   ```bash
   BLOG_WECHAT_BROWSER_FIXTURE=/tmp/blog-wechat-fixture.url go test ./internal/data -run TestOfficialAccountHTTPLifecycle -count=1 -v
   ```

2. 在 `frontend/` 构建产物，再启动服务（不要同时运行 dev）：

   ```bash
   pnpm build
   NUXT_BACKEND_BASE="$(cat /tmp/blog-wechat-fixture.url)" NITRO_HOST=127.0.0.1 NITRO_PORT=13000 node .output/server/index.mjs
   ```

3. 仓库根目录运行（如需本机 Chrome，用 `BLOG_E2E_CHROMIUM` 指定路径）：

   ```bash
   NODE_PATH=<playwright所在的node_modules目录> node scripts/e2e/wechat_sync_e2e.cjs
   ```

4. 完成后新建 `/tmp/blog-wechat-fixture.url.stop` 通知 fixture 正常退出，并停止该前端进程。重复验收先移走上一轮这个精确 stop 文件；测试最多保留五分钟，退出后内存数据库与示例账号均消失。

## 域名 HTTPS 冒烟

生产切换后运行 `python3 scripts/e2e/domain_smoke.py`。默认 https://makerhao.cn，系统 CA 验证证书；检查 ICP 页脚、canonical、公开 BFF API、登录 noindex、匿名退出的 Secure cookie 与 Path 重写。真实登录/刷新和微信真机验证仍需另行执行。切换步骤见 `docs/operations/runbooks/domain-cutover.md`。
