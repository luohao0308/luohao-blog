# 小程序身份与绑定

## 身份契约

`auth.proto` 的 HTTP/gRPC 请求形状保持不变：登录只提交微信临时 code，绑定提交一次性票据和网站邮箱密码。可信 AppID 只取服务端配置，不接受客户端指定身份来源。

`wechat.enabled` 默认 false；只有显式启用且 AppID/Secret 完整时，登录与绑定才开放。禁用时两者均返回 412，且不消费票据。公众号 `official_account` 配置独立。

v16 `wechat_identities` 保存 `user_id/app_id/openid/created_at`；`(app_id,openid)` 与 `(user_id,app_id)` 分别唯一，大小写敏感。新登录只查该表，不回退旧 `users.wechat_openid`。旧列不删除、不改写。不同 AppID 下相同 OpenID 字符串不能作为账号自动合并依据。

绑定使用网站密码验证结果的 credential version，在同一 MySQL 事务锁定用户并复核版本后插入身份。精确重复幂等；身份属于其他用户、同用户同 AppID 已有其他身份均返回 409，不覆盖旧关系。密码版本失配返回 401。签发会话后再次复核凭据版本，发生变化则撤销刚创建的刷新会话并返回 401。

Redis 票据命名空间为 `blog:wechatbind:v2:<appID>:<opaque-ticket>`，载荷包含 AppID/OpenID；有效期最多 10 分钟，GETDEL 原子消费。旧格式、跨 AppID、过期、重放及畸形载荷拒绝。错误密码消耗本次票据，需重新微信登录。

## 迁移与回填契约

v16 只创建新表，不自动猜测旧 AppID 或搬移旧身份。`wechat-identity-backfill -conf <private-directory> -legacy-app-id <verified-app-id>` 默认事务回滚的 dry-run；实际提交还需 `-apply -writers-paused`。命令只接受 clean v16，不执行迁移；回填精确相同关系可重复执行，归属冲突整批回滚。输出仅数量，不输出身份、DSN 或凭据。

生产操作顺序、停写和故障边界见[接入 Runbook](../operations/runbooks/formal-miniprogram.md)。小程序跨 AppID 的本地收藏不自动搬移。

## 可执行检查

- `backend`: `go test ./internal/biz ./internal/data ./internal/service`，含禁用、身份隔离、票据和 HTTP 负向场景。
- 仓库根：`python3 scripts/test_wechat_identity.py`，使用独立 Docker MySQL/Redis，执行真实 v15→v16、旧数据保留、CLI dry-run/apply/幂等、升级后备份恢复、实际锁/唯一约束/CAS 和 HTTP/Redis 会话清理；缺少指定真实驱动用例或出现 Skip 均失败。占用 loopback 13316/16389，默认创建并仅清理本次容器；不读取生产配置。
- 小程序：`npm run typecheck`、`npm test`。正式 AppID 的真机与微信平台结果属于后续验收，不能由本地测试替代。
