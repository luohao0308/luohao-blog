# 架构决策记录（ADR 索引）

_状态：active | 更新：2026-09-30_

## 决策索引

| ID | 决策 | 状态 | 日期 | 影响范围 | 替代/关联 |
|---|---|---|---|---|---|
| ADR-0001 | 认证 token 模型：短效 access JWT + Redis 会话化 refresh token | accepted | 2026-09-30 | backend/auth、S3 RBAC、S4 前端 | M2 计划 §5 决策表 |
| ADR-0002 | 迁移治理：golang-migrate 版本化 SQL 替代 ent auto_migrate | accepted | 2026-09-30 | backend/migrations、data 启动流程 | M2/S1（PR #6） |

## ADR 模板

### ADR-XXXX：<!-- 标题 -->

- 状态：proposed / accepted / superseded
- 日期：YYYY-MM-DD
- 背景与证据：
- 决策驱动因素：
- 采用方案：
- 考虑过的替代方案：
- 结果与代价：
- 兼容、迁移和回滚影响：
- 验证方式：
- 后续复审触发条件：

> 不删除已经生效的历史决策。方案被替代时，把状态改为 `superseded` 并链接新 ADR。

### ADR-0001：认证 token 模型

- 状态：accepted
- 日期：2026-09-30
- 背景与证据：M1 文章写接口完全无鉴权；M2 目标"只有持有效凭证的作者能写文章"。kratos v3 无现成 JWT 中间件（`middleware/` 仅 recovery/validate/selector 等，`middleware/auth` 不存在，已在 module cache 中核实）。
- 决策驱动因素：刷新可撤销（登出/封号即时生效）、大厂通行模型、个人博客无第三方登录需求、kratos v3 生态缺口可自研极简中间件。
- 采用方案：access token 为 HS256 JWT（15min，claim：sub=uid、role、exp/iat），无状态校验；refresh token 为 256bit 不透明随机串，服务端会话存 Redis（`blog:auth:refresh:<token>`，7d TTL 滑动），经 httpOnly cookie（Path=/v1/auth）下发，刷新时 GETDEL 原子消费实现单次使用旋转；登录按客户端 IP 做固定窗口限流（默认 10 次/5min，Redis INCR+EXPIRE，限流器故障 fail-open）。JWT 中间件自研（`internal/server/auth.go`）：受保护操作清单匹配 operation id（HTTP 路径模板 / gRPC 全方法名），过期与无效 token 区分 reason（AUTH_TOKEN_EXPIRED / AUTH_UNAUTHORIZED），公开接口上的坏 token 降级为匿名放行。
- 考虑过的替代方案：kratos 官方 jwt 中间件（v3 未提供）；纯 JWT 双 token（登出无法即时撤销）；服务端会话 cookie 全托管（前端内存态 access token 的方案更契合 SPA + SSR 代理）。
- 结果与代价：需要维护自研中间件与 Redis 会话存储；JWT 吊销有最长 15min 的滞窗（可接受，refresh 即时吊销）；限流信任 X-Forwarded-For 首跳，直连部署下可被伪造（见 PROJECT-SUMMARY 风险表）。
- 兼容、迁移和回滚影响：auth 为新增契约，无破坏；S3 将 article 写接口加入受保护清单属破坏性变更（PR 描述标注）；回滚 = revert PR，Redis 会话可整体 FLUSHDB 按需吊销。
- 验证方式：单测（签发/过期/伪造/alg-confusion/无 exp、会话旋转/重放/过期/删除账号、限流窗口/默认值、中间件 401/匿名放行）；真库冒烟 `backend/scripts/smoke-auth.sh` 30/30 通过（2026-09-30）；空密钥启动 fail-fast 实测。
- 后续复审触发条件：引入第三方登录/多用户注册；部署到 TLS 域名（补 cookie Secure）；出现 token 滞窗被利用的实际风险。

### ADR-0002：迁移治理

- 状态：accepted
- 日期：2026-09-30
- 背景与证据：M1 期间依赖 ent auto_migrate 建表，schema 变更无评审、无版本。
- 决策驱动因素：用户指定 golang-migrate；SQL 显式可评审、面试通用。
- 采用方案：迁移 SQL 进 `backend/migrations/`（embed 进二进制），启动时幂等 up；auto_migrate 停用。
- 考虑过的替代方案：Atlas 版本化迁移（ent 亲和，但引入额外工具链）。
- 结果与代价：ent schema 与迁移 SQL 双源需人工对齐，靠迁移演练与基线比对缓解。
- 兼容、迁移和回滚影响：迁移可重放；回滚按 down 文件。
- 验证方式：M2/S1 真库 up/down/up 演练；每次启动自动 up 幂等。
- 后续复审触发条件：出现大规模回填需求。
