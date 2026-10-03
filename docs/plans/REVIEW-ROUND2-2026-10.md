# 第二轮 Code Review 记录（2026-10-04）

_范围：PR #28 修复代码对抗审查 + data/RAG 层深挖 + 契约与运维。三个 Explore 子代理 + 本机自查。本轮无 P0。_

## P1（2 项）

1. **useAuth 登出与 in-flight 刷新竞态（会话"复活"）** — `useAuth.ts:137-144` + `biz/auth.go:156-181`：logout 只 clearSession 不撤销 in-flight refreshOnce；时序（401→刷新轮换出新会话 S2→登出携带旧 cookie 是 no-op→refresh 响应 applyReply 复活状态 + S2 Set-Cookie 晚到）→ 刷新页面后管理员会话完整复活（"假登录"，单飞修复的镜像 bug）。修法：logout 先 await refreshOnce（吞错）再撤销清态，或引入 session epoch。
2. **运维资产在版本库外** — watchdog.sh 仅存在于服务器（untracked、cron */5 分钟）、backup cron 仅在服务器 crontab、**docs/ 与 AGENTS.md 整体未入 git**（git log --all -- docs/ 为空）。服务器/磁盘丢失即失传。修法：watchdog.sh 拷回 deploy/ 入库 + docs/AGENTS.md 提交推送 + runbook 补"定时任务"节（确切 crontab + 验证命令）。附带：备份单机存储无异地副本、cron 静默失败无告警。

## P2（12 项）

| # | 发现 | 位置 |
|---|---|---|
| 1 | LLM 超时 120s 被 HTTP server 60s deadline 截断——reasoning 模型下聊天 60s 必失败（两者注释互相矛盾） | `data/llm.go:41-46` vs `configs/config.yaml:5-7` |
| 2 | kratos errors.Error 实际走 protojson（内嵌 Status 满足 proto.Message），codec.go"stdlib 形状"注释论断错误；EmitUnpopulated 给错误体加 `metadata:{}`；非法 UTF-8 → Marshal 失败 → 裸 500 空体；现测试无法区分两条路径 | `server/codec.go:47-51`、`codec_test.go:49-65` |
| 3 | page_token 只修了解析失败，**合法格式的巨大 offset** 直通 ES `from`（from+size>10000 → 500，正是 #28 要封的 500 类）与 MySQL OFFSET | `service/search.go:45`、`data/es.go:335` |
| 4 | openapi.yaml 属性名 camelCase vs 线上 snake_case（#28 UseProtoNames 后系统性漂移；int64/时间戳/枚举本就对齐） | `buf.gen.yaml:24-28` 生成管道 |
| 5 | release.yml 无 concurrency（两次快速合并旧构建反向覆盖 ghcr latest）+ 无测试门禁（main 直推可绕过 CI 出镜像）；两 workflow 均无 timeout-minutes | `.github/workflows/release.yml:17-19` |
| 6 | update_mask 引用未知字段静默忽略（proto/openapi 承诺 400）；einride 对未知 path 直接 return | `service/article.go:146-158` |
| 7 | 搜索契约宣称 "Chinese analyzed"，实际 standard analyzer 单字切分 | `data/es.go:18-29`、`search.proto` |
| 8 | Caddy 无访问日志（限流攻击排查/审计无原始记录）；restore.sh 边服务边恢复（应先 stop backend）；容器日志无轮转（json-file 无上限） | `caddy/Caddyfile`、`restore.sh:11`、`compose.prod.yml` |
| 9 | backup.sh：mysqldump 早期失败产生合法空 gzip，restore 的 gzip -t 挡不住"空库假成功"（需 trap 删残骸 + dump 头校验） | `backup.sh:12`、`restore.sh:9` |
| 10 | E2E 套件 known_issue 标注已失效（P0 已修，若回归会 exit 0 静默放行）+ 一轮消耗 7 次登录限流、连跑必撞 429 | `scripts/e2e/blog_e2e.py` |
| 11 | demo 文章软删后 seed 不再幂等（存在性判断排除 DELETED → slug 冲突错误退出） | `cmd/seed/demo_articles.go:146-167` |
| 12 | README.md / PROJECT-SUMMARY.md 严重漂移（停在"M4 进行中、M5 未立项"，实际 RAG 已上生产）；README 技术栈含未引入的 Prometheus/MinIO 表述 | 两文档 |

## P3 长尾（摘录，全量见各代理报告要点）

- codec：`Accept: application/protojson` 命中 kratos 内置 protojson（无 UseProtoNames）→ 契约对齐仅对 Accept: json 成立；建议 server 强制编码器
- 前端类型漂移：`expires_in`/`view_count` 运行时 string、TS 声明 number（现仅渲染无算术）
- clientIP 最右跳未做 `net.ParseIP` 规范化（带端口 hop 会碎片化限流桶；当前 Caddy 行为下不触发）
- BFF 直连（仅本地/dev 面）XFF 全客户端可控；gui-test-screenshots/ 未 ignore；backend .dockerignore 排除 .git 后镜像 VERSION 变空串
- E2E 脆弱点：t15 finally 的清理 assert 会顶掉原异常；t14 删除不在 finally；t11 外部 LLM 依赖未标注
- ES：启动路径 context.Background() 无超时可挂起；ES 深分页/kNN 硬限制静默降级为空结果；tags 进 multi_match 基本无效（keyword 全串匹配）
- schema 双源：ent 注释 VARCHAR(191) vs 迁移 varchar(255)；`content` ent Optional vs 迁移 NOT NULL；TIMESTAMP 2038 上限；"SQLite 测试通过 ≠ MySQL 行为"的结构性根因
- reindex 只取 1000 篇 + 串行 + 无 deadline + 重建窗口搜索静默为空；seed 不回填 ES 索引（新部署需手动 reindex，无提示）
- 软删除占坑 slug 永久不可复用（可辩护但未文档化）；`auto_migrate` 死配置；限流阈值/fail-open 语义未进任何契约；评论 admin 列表排序无 tiebreaker；next_page_token 可指向空页；chat 无缓存重复烧 token；用户问题未加 prompt 定界符（实际风险低：citation 服务端固定）；LLM 响应体无大小上限、错误响应不排干连接
- 密码强度只有下限 8 位（无上限，argon2 DoS 面）

## 值得补测试的优先级

1. es.go 全文件（httptest 覆盖 mapping/搜索/降级）2. 限流器 EXPIRE 失败恢复 3. IncrementView fail-open 分支 4. 真实 MySQL 迁移+冲突+软删+分页集成冒烟（封 schema 双源差异）
