# 计划：Code Review 修复（2026-10 review 发现）

_status: 全部切片已完成并验证（2026-10-04），分支 `fix/code-review-2026-10` 本地 7 commits，待用户授权 push/PR_
_来源：2026-10-03 全量代码 review + Playwright E2E 验证（证据：gui-test-screenshots/e2e-py/）_

## 执行记录（2026-10-04）

| 切片 | 提交 | 验证 |
|---|---|---|
| S1a 前端 tags 判空 | `932b684` | E2E t01/t02/t03/t09 全 PASS |
| S1b 后端 protojson codec | `aede208` | 单测 5 项 + 线上格式核验（tags 恒在/枚举数字/时间戳 RFC3339） |
| S3 clientIP 最右 XFF | `900bf0e` | 单测 7 例（伪造首跳/多跳/空值） |
| S4 搜索 400 + page_size 上限 | `fdfa6f2` | 单测 + 实弹 curl（bad token 400、page_size=100000→200） |
| S4 useAuth 单飞刷新 | `cc24b32` | E2E 登录/登出/全管理流回归通过 |
| S2 泄漏面收口 | `b11461b` | chmod 600 核验、git check-ignore 核验、bash -n |
| E2E 套件入库 | `c48860f` | 16/16 PASS（含 t13 等待修正） |

备注：backend 镜像本机构建时 compose 里的腾讯内网 APT_MIRROR 在 Docker VM 不可达，
本地用 `docker compose build --build-arg APT_MIRROR=deb.debian.org backend` 绕过；
服务器上 compose 原值不受影响。

## 切片

### S1 P0：无标签文章打挂全站（tags 未判空）
- **目标**：任意数据态下首页/列表/标签页/搜索/后台列表不再崩溃。
- **改动**：前端 3 文件 5 处 `?.`/`?? []` 防御（index.vue:66、ArticleCollection.vue:35/37/38、admin/posts/index.vue:113）；后端注册 protojson 语义的 "json" codec（UseProtoNames+UseEnumNumbers+EmitUnpopulated，非 proto 载荷回退 stdlib），空 repeated 字段输出 `[]`，与 openapi 契约对齐。
- **验收**：go test 全绿；pnpm lint/typecheck/build 全绿；E2E t01/t02/t03/t09/t13 由已知缺陷复现转 PASS；全站冒烟。
- **回退点**：单分支多提交，revert 对应 commit 即可。

### S2 P1：泄漏面收口
- **改动**：新增 backend/.dockerignore（排除 secrets）；根 .gitignore 补 `backups/`、`!deploy/.env.prod.example`、`__pycache__/`；本机密钥/备份文件 chmod 600；backup.sh 加 umask 077；restore.sh 恢复前 gzip -t 完整性校验。
- **验收**：`git status` 不再出现 backups/；`git check-ignore` 核验；脚本 bash -n；（示例模板是否 git add 由用户在 PR 时决定）。

### S3 P1：限流键防伪造
- **改动**：`service/clientIP` 改取 X-Forwarded-For 最右一跳（可信反代追加的真实地址），缺席回退 RemoteAddr；补纯函数单测。
- **验收**：单测覆盖多跳/伪造/空值；冒烟登录/评论限流正常。
- **不在本轮**：Secure cookie —— buf 本机缺失无法重生成 conf.proto，且站点仍为 HTTP，开了即断登录；等域名+TLS 落地时以配置项实现（Runbook 待补）。

### S4 P2：边界加固
- **改动**：搜索 page_token 解析错误映射 400（对齐 article/comment 既有模式）；三个 list 接口 page_size 上限 100；useAuth refresh 单飞（并发 401 共享一次轮换，修复假登出）+ ensureSession 失败后可重试。
- **验收**：service 层单测（page_token→400、page_size 截断）；E2E 全量回归。

## 交付
单分支 `fix/code-review-2026-10`，按切片分 commit；push/PR 需用户逐次授权（manifest manual+user）。
