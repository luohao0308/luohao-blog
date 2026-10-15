# 网站能力补齐交付与恢复

适用于网站补齐切片 S1–S6，包含迁移 12–15。实现和本地验收见 [计划](../../plans/WEBSITE-COMPLETION-2026-10.md)、[账号邮箱契约](../../contracts/ACCOUNT-EMAIL.md)、[作品与关于契约](../../contracts/PORTFOLIO-ABOUT.md)、[收藏契约](../../contracts/COLLECTIONS.md)及 [E2E 复跑指南](../../../scripts/e2e/README.md)。本地通过不代表生产已经更新。

## 前置条件与 Preflight

远端 Git、镜像发布、生产部署、迁移或回滚分别绑定准确版本和动作，按项目交付策略取得授权并运行 delivery guard。不得推送 `codex/*` 临时分支或直接推送受保护 main。完成远端 required CI 后，记录实际交付 SHA 与镜像版本，再准备对应环境的部署动作；禁止用可变化的分支名代替批准版本。

发布前重新验证生产 Git/镜像、健康状态、数据库版本和 dirty 状态、uploads 卷挂载、备份空间与恢复入口。备份同时包含 MySQL 和 uploads，并验证完整性；只有任务隔离 MySQL 的恢复演练可直接重跑，不能把测试脚本指向生产。

新增数据结构均为扩展式：v12 增加文章封面/精选；v13 增加作品与 About 幂等种子；v14 增加邮箱验证、凭据版本和目的 token 表；v15 增加收藏表。旧账号和文章保留，旧账号邮箱验证状态初始 false；不批量声明旧邮箱已验证，不硬删隐藏收藏或旧媒体。

## 部署及最小验证

1. 在批准窗口先备份并校验，确保没有脏迁移。后端启动会应用其嵌入的 up migration，故启动新后端本身包含迁移动作，需要对应授权。
2. 按批准的准确版本更新后端和前端；保留持久 uploads。确认数据库迁移版本 15、dirty=false，以及新旧文章、账号、头像和公众号草稿配置仍可读。
3. 看门狗采用 Docker health 状态探针，不能再访问未映射的宿主端口。更新脚本前检查目标容器名称与 healthcheck；需要暂停自动重启时使用 `BLOG_WATCHDOG_RESTART_ENABLED=0` 只告警，而非恢复旧探针。
4. 验证首页、文章、作品、About、robots、XML sitemap 与公开图片；检查私密页面 noindex，草稿/下架内容不在公共接口或 sitemap。用受控测试账号验收后台分页、图片、作品与 About 保存、两浏览器收藏、显式导入及退出隔离。不得以匿名首页 200 替代这些闭环。
5. 邮件默认保持关闭。没有真实发信身份与投递验收时，应记录 `pending_external`；验证/找回入口的未配置状态必须如实显示，旧登录继续可用。

## 配置真实邮箱服务

本机使用 `backend/configs/secrets.local.yaml`，生产 compose 挂载 `deploy/prod-conf/secrets.local.yaml`。参考同目录的 `secrets.local.example.yaml`，只合并 `mail` 段到已有私密文件，保留原 JWT、数据库和微信等配置，不能复制模板覆盖已有文件。真实密码只在这些 ignored 文件或批准的服务端环境中保存，不能贴到对话、提交源码或输出配置全文。

DirectMail 控制台完成发信域名所有权、SPF、DKIM、DMARC、MX 验证后，创建发信地址和该地址的 SMTP 专用密码。配置 `mail.username`、`password`、`from_address`；用户名是发信地址。配置可信 `public_site_origin` 为实际 HTTPS 博客 origin，检查区域匹配的 `smtp_host`，使用 465 隐式 TLS、证书验证和总操作超时。全部配置可用后才将私密覆盖中的 `mail.enabled` 改为 true；配置更改需批准并重启对应后端实例。

真实联调只向明确批准的自有邮箱发一份验证邮件和一份找回邮件，检查收件、fragment 链接、一次确认、旧刷新会话拒绝、新密码登录；不要对访客邮箱做探针，不盲重发或假称投递成功。密码重置会撤销 refresh 与恢复 token；已签发 access JWT 可继续到其配置 TTL，默认最长约 15 分钟。找回请求只报告受理，后台发送失败仍需查看不含收件人/token/provider敏感内容的诊断。

## 失败处理与回退

应用异常先停止当前发布，保留数据库与媒体。可以回退前端界面或关闭邮件配置，同时保留能够识别当前迁移版本的后端。不能简单回到没有 v12–v15 migration 源的旧后端：golang-migrate 启动时会验证当前版本在嵌入源中存在，旧二进制可能拒绝启动。

不得用 down migration 丢弃新表/字段，也不得通过 force version 掩盖错误。若必须恢复升级前数据库，需单独授权准确备份与版本、停止所有写入方、说明升级后新增记录如何保留或补回，并按既有恢复 Runbook 验证。SQL dirty、损坏备份或版本不匹配时先修复证据与恢复方案，不能继续自动切换。

本地 MySQL runner 已验证 11→12/13/14/15、12→13、13→14、14→15，以及升级前备份恢复重升级和升级后验证状态/token/收藏记录的恢复；重复作品/About seed 不覆盖后台编辑。生产恢复不能仅凭本地演练宣布通过，需在批准的具体环境核验。

## 证据入口

代码验证见 required CI；本地可执行检查见计划和 `scripts/e2e/README.md`。`scripts/test_mysql_migrations.py` 仅接受任务专属 label、volume、loopback 端口的 MySQL，并断言仓储测试实际 RUN/PASS，禁止默默 Skip。一次性运行输出留在受控 artifacts 或 ignored 工作目录；本文件不保存部署流水、凭据或收件人。
