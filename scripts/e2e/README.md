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

## 网站能力补齐专项

`admin_pagination_e2e.cjs` 使用120篇模拟文章检查后台分页、筛选、修改后补位及错误恢复；`article_experience_e2e.cjs` 验证媒体插入、封面/精选、阅读目录和移动端；`portfolio_about_e2e.cjs` 验证后台维护、公开上下架隔离、页面切换及错误重试。模拟接口用例不能替代真实接口验收。

真实接口用例 `article_media_live_e2e.cjs` 和 `portfolio_about_live_e2e.cjs` 不拦截 API，使用独立 SQLite/miniredis/临时文件 fixture，通过 Nuxt BFF 完成登录、写入、刷新、匿名回读及越权拒绝。只允许本机地址，不读取生产配置、不联系真实微信接口。

1. 先构建前端；构建完成后，在后端启动对应 fixture（避免五分钟 fixture 窗口被构建占用）：

   ```bash
   BLOG_PORTFOLIO_BROWSER_FIXTURE=/tmp/blog-portfolio-fixture.url go test ./internal/data -run TestOfficialAccountHTTPLifecycle -count=1 -v -timeout=6m
   ```

2. 按上述公众号 fixture 步骤启动预览，将 `NUXT_BACKEND_BASE` 指向这个 URL 文件；在根目录运行对应脚本，指定相同 `BLOG_E2E_BASE_URL`、已安装 Playwright 的 `NODE_PATH`，必要时指定 `BLOG_E2E_CHROMIUM`。
3. 完成后创建精确的 `/tmp/blog-portfolio-fixture.url.stop` 文件并停止本任务预览。再次启动时使用新 URL 文件，避免旧 stop 信号。fixture 正常退出只证明其自身 HTTP 测试通过；浏览器必须另有脚本 `PASS` 结果。

文章呈现工具边界用 `node --experimental-strip-types scripts/test_article_presentation.mjs` 验证。看门狗隔离回归用 `python3 scripts/test_watchdog.py`，不会重启真实 Docker 容器。真实 MySQL 升级、旧行兼容、幂等内容导入及 SQL 备份恢复用 `python3 scripts/test_mysql_migrations.py --baseline 12 --target 13`；该命令仅接受任务专属 label/volume/loopback 端口的隔离 MySQL，私密测试配置不得提交。

SEO专项在前端构建后运行 `NODE_PATH=<已有Playwright目录> BLOG_E2E_CHROMIUM=<已有Chrome路径> node scripts/e2e/seo_e2e.cjs`。脚本自行启动并停止仅绑定loopback的mock后端和随机端口Nitro preview，固定测试canonical为https://makerhao.cn，不访问该真实域名。验证121文章/111作品的完整分页、公开过滤、下架新鲜地图、无调用者会话转发、XML解析、SSR/head字段以及重复游标、空页续页和上游挂起503。工具边界用 `node --experimental-strip-types scripts/test_seo.mjs` 验证固定origin、受控图片、XML转义和时间戳。

账号邮箱专项：`account_email_e2e.cjs`使用模拟接口验证503/429/确认重试、fragment清理、其他账号token确认隔离、默认JWT残余窗口文案以及后台改密入口复用/当前会话退出。`account_email_live_e2e.cjs`不拦截API，使用真实BFF、SQL、Redis和fakeMailer；它动态注册三个仅fixture账号，确认验证/重置单次消费、目的隔离、弱密码不消费、旧refresh失效、其他账号会话保留、新密码登录及未验证邮箱重置证明。两者都只允许loopback，不发送真实邮件。

先build前端，再在backend执行 `BLOG_ACCOUNT_EMAIL_BROWSER_FIXTURE=/tmp/blog-account-email-fixture.url go test ./internal/data -run '^TestAccountEmailHTTPLifecycle$' -count=1 -v -timeout=12m`，将该URL用于NUXT_BACKEND_BASE并启动preview；按其他专项同样设置BLOG_E2E_BASE_URL/NODE_PATH/Chrome运行live。`/fixture/emails/latest`只在测试服务捕获邮件链接，脚本只在内存读取token并在错误中脱敏，禁止把捕获link/token打印到终端。最后建立精确URL文件的`.stop`并停止自有preview；fixture最长10分钟。MySQL13→14/11→14 runner自动传受控DSN给`TestAccountEmailTokenRepoMySQL`并断言真实运行而非Skip；不使用生产配置。
