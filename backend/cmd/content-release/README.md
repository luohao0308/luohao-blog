# content-release

一次性 creator bundle 发布工具，从 backend/ 运行：

```sh
go run ./cmd/content-release -conf ./configs -bundle ../content/releases/creator-content-20261028.json
go run ./cmd/content-release -conf ./configs -bundle ../content/releases/creator-content-20261028.json -apply -bundle-sha256 <经批准的完整文件SHA256> -backup /受控目录/creator-before.json
```

默认 preflight：严格检查 manifest、全部 13 项时间戳/slug、proto 类型、现有业务校验与 Markdown 渲染，不进行内容写入。create 使用 SQL COUNT 检查所有同 slug 行，包含软删除。仅 apply 要求准确文件 hash 和新备份文件路径；已有文件（含符号链接）拒绝覆盖。备份包含八个现有目标公共对象的完整 proto 快照，0600，落盘同步后才开启实际写入；另外五个 create 尚无快照。全库备份由发布负责人另行完成。

使用 seed 相同的 Kratos file + KRATOS env 配置来源。调用自动迁移的 NewData **之前**，通过独立 database/sql MySQL 连接只读查询 schema_migrations，必须恰好一行、dirty=false、版本等于 migrations.FS 内最高 .up.sql（当前 16）；缺表、空表、旧版、超前版、dirty 均拒绝。每项实际写入前再次验证 schema 与目标时间戳/slug。NewData 本身仍执行现有 migration runner 的无待执行检查和锁；操作期间必须避免并发部署/迁移。

更新文章只使用 title/summary/content_md/tags mask；bundle 的 status=2 对更新不生效。新文章先按现有接口建立 DRAFT，再重读版本并通过 status mask 发布。项目保留 cover/screenshots；关于先读完整对象，再替换六个获准字段，保留 id/profile_image/contacts/representative_links。未连接 Redis，不创建账号、会话、token，不触发微信发布。

ES 配置必须存在，所有配置地址的既有索引 HEAD 必须成功。默认 preflight 仅 HEAD，不调用可能创建索引的 NewEsIndexer。apply 在备份后按 reindex 的配置方式构建现有 indexer，注入 ArticleUsecase；只增量同步本次文章，最后 Refresh，**无需运行会 drop 索引的 reindex**。捕获业务层原本只记录警告的索引失败并停止。ES/DB 非事务：索引失败时当前文章可能已保存，必须人工核对；不重试、不回滚。现有 embedding 失败降级 BM25 的规则沿用。

逐项时间戳检查与现有 service 保存之间不提供数据库 CAS；发布窗口应独占内容编辑。错误仅输出通用失败信息，正常输出仅 kind/slug/status/count。框架、SQL driver 和 ES 日志被抑制，禁止打印底层错误或配置。

本目录测试不连接数据库、ES、生产或 SSH：

```sh
go test ./cmd/content-release
go vet ./cmd/content-release
go build -o /tmp/content-release ./cmd/content-release
```

本机真实 MySQL 回归（仅允许 127.0.0.1 上独立 creator_content_test 数据库）：

```sh
BLOG_CONTENT_RELEASE_MYSQL_DSN='root@tcp(127.0.0.1:13328)/creator_content_test?parseTime=true&multiStatements=true&loc=Local' go test ./cmd/content-release -run TestMySQLCreateReadPublish -count=1
```

MySQL 秒精度 TIMESTAMP 与 Ent 创建返回值可能有舍入差异，创建后比较限定在一秒内，并同时核对身份、草稿状态、正文、标签与媒体值；可选 proto 字段按值比较。

## 六项作品集正文修订

显式使用 `-revision` 读取独立 schema；默认旧 13 项 creator/reader bundle 校验不变，两个模式不接受对方文件。

```sh
# 仅预检，不进行内容写入
go run ./cmd/content-release -revision -conf ./configs -bundle ../docs/content/portfolio-revision-2026-10.json
# 执行只用于已获精确授权的环境和文件；hash 绑定完整原始字节
go run ./cmd/content-release -revision -conf ./configs -bundle ../docs/content/portfolio-revision-2026-10.json -apply -bundle-sha256 <已核准SHA256> -backup /受控目录/portfolio-before-new.json
```

新模式固定四个项目（aigc、forge-harness、dev-workflow、luohao-blog）与两篇文章（aigc-backend-delivery、forge-harness-project），绑定提案 baseSha；不存在 create、about 或发布路径。项目 mask 只允许 name/summary/description/stack/role，文章只允许 title/summary/content_md；before、after 与去重 mask 的键必须完全一致。JSON 类型、额外字段、目标重复、错误时间戳或缺失保留字段均拒绝。

全部六项先执行身份、updated_at、每个 before 和 preserved 字段核对，并使用既有业务校验及 Markdown 渲染完成无写入预检，再写入新的 0600 O_EXCL 完整对象备份。每项实际更新前重复 schema 与字段核对；更新后重读对象，核对 ID、after 及 preserved。文章状态、首次发表时间、标签、分类、封面和 featured，项目发布状态、封面、截图、源码地址、体验地址及其他提案保留字段均不进入更新 mask。

仍沿用 schema 16 clean、既有 ES HEAD、仅本次两篇文章增量索引及最终 Refresh；不连接 Redis、不创建账号、会话或发布任务。任何失败立即退出，无盲重试、自动回滚或续跑。全部预检后备份不代表跨项目/文章事务：数据库 CAS 和 DB/ES 事务仍不存在，必须独占 CMS 编辑窗口；中途失败后由发布负责人核实已提交的实际状态。
