# 账号邮箱验证与密码恢复

机器契约以 `backend/api/blog/v1/auth.proto`、`internal/conf/conf.proto` 和迁移 `000014_email_verification_password_reset` 为准。本文件记录安全边界与运维前提，不保存真实发信凭据。

## HTTP 契约

- `POST /v1/auth/email-verification/request`：登录账号请求本人验证邮件。
- `POST /v1/auth/email-verification/confirm`：公开提交 `{token}`。
- `POST /v1/auth/password-reset/request`：公开提交 `{email}`。配置正常时，已注册与未注册邮箱统一返回空成功响应；界面只能提示“若邮箱已注册且投递可用，将收到链接”。
- `POST /v1/auth/password-reset/confirm`：公开提交 `{token,new_password}`。

`User.email_verified` 初始为 `false`。未验证账号仍可沿用原有登录、评论和微信绑定。普通旧密码改密不会把邮箱标记为已验证；邮件重置成功会标记为已验证。

验证链接有效 24 小时，重置链接有效 15 分钟。链接把 token 放在 URL fragment：`/verify-email#token=...`、`/reset-password#token=...`，服务端不从请求 Host 构造链接。

## 状态与安全

数据库只保存 SHA-256 token hash。`(user_id,purpose)` 唯一，因此每种用途每个账号只有一个当前 token；重发会使旧 token 失效。确认操作在一个数据库事务中完成 token 单次删除、credential version 条件更新及状态改变，失败回滚后 token 仍可重试。

密码仍使用项目既有 Argon2id。新密码为 8–1024 UTF-8 bytes；历史长密码登录不增加新上限。密码改变会增加 `credential_version`，旧验证/恢复 token 和 refresh session 随即逻辑失效；现有 access JWT 最多继续有效到配置的短 TTL（默认 15 分钟）。

恢复请求使用单 worker 的有界内存队列，HTTP 请求不按账号存在性同步等待 SMTP。确认重置也有容量 1 的 Argon2 门，非法 token 在哈希前被廉价拒绝。

限流依赖 Redis并 fail closed：验证重发冷却 60 秒、每账号每小时 5 次；恢复每邮箱每小时 3 次、每 IP 每小时 5 次。Redis 不可用不会绕过恢复限流。

## DirectMail 配置

默认关闭。复制私密配置模板，填入发信地址、SMTP 专用密码与可信站点 origin，完成域名验证后再将 `mail.enabled` 改为 `true`。实现仅接受 465 隐式 TLS，最低 TLS 1.2，验证证书与 SNI，不降级到明文 SMTP。

阿里云官方资料：

- [SMTP 服务地址与端口](https://help.aliyun.com/zh/direct-mail/smtp-endpoints)
- [发信域名配置](https://help.aliyun.com/zh/direct-mail/user-guide/configurations/)
- [Go SMTP 调用说明](https://help.aliyun.com/en/direct-mail/smtp-go)

新发信域名需在控制台完成所有权、SPF、DKIM、DMARC 与 MX 验证。SMTP 用户名是控制台发信地址，密码是该地址的 SMTP 专用密码。

本地测试使用注入 sender 或 loopback TLS SMTP，不访问真实 DirectMail。只有真实邮箱收到邮件并完成一次闭环后，才能把真实发信标为上线。
