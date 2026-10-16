# 契约治理入口

_状态：M1/S1 起维护 | 权威范围：对外可观察的 API、事件、Schema、CLI 和文件格式 | 更新：2026-09-29_

## 权威顺序

1. 已验证的运行中行为和机器可解析契约；
2. 生成机器契约的源代码或 Schema；
3. 契约测试与消费者测试；
4. 人工使用指南和示例；
5. 历史快照与归档。

人工指南不能悄悄覆盖机器契约。出现冲突时先验证当前实现，再同步修正文档和生成物。

## 契约索引

| 契约 | 类型 | 生产者 | 消费者 | 权威源 | 生成/验证入口 | 兼容策略 |
|---|---|---|---|---|---|---|
| ArticleService（文章 CRUD，HTTP+gRPC+OpenAPI） | HTTP/gRPC | backend Kratos `ArticleService`（internal/service/article.go） | Nuxt 前端（`/api` 代理，S4）、站点读者 | `backend/api/blog/v1/article.proto`（唯一源） | `make api`；生成物 `*.pb.go`、`openapi.yaml`，`go build ./...` 兜底 | M1 无外部消费者，可自由演进；S1 登记后变更走 CHANGE-CHECKLIST |

| OfficialAccountService（同步配置状态、任务分页、重试、显式同步） | HTTP/gRPC | backend OfficialAccountService | Nuxt admin/wechat-sync | `backend/api/blog/v1/official_account.proto` | `make api`；HTTP 权限/流程测试 `TestOfficialAccountHTTPLifecycle` | 新增管理员接口，未配置时不产生远端调用 |

| MediaService（文章图片上传与公开回读） | HTTP/gRPC | backend MediaService | Nuxt ArticleForm、文章/作品封面 | `backend/api/blog/v1/media.proto` | `make api`；media权限/FS/HTTP测试、S2 browser及真实BFF回归 | 新增后台上传/公开资产接口，2MiB内容和4MiB请求限制；Article可选metadata旧mask保留 |

操作边界见 [文章媒体与元数据](MEDIA-ARTICLE-METADATA.md)。

| ProjectService / AboutService | HTTP/gRPC | backend Project/About services | Nuxt public与admin维护页 | `project.proto` / `about.proto` | `make api`；repository/HTTP权限与S3 browser，MySQL迁移恢复 | v13扩展新表、原内容幂等seed，草稿隔离、Project软下架 |

详情见 [作品集与关于维护](PORTFOLIO-ABOUT.md)。

| AuthService 邮箱验证与密码恢复 | HTTP/gRPC | backend AuthService / AccountEmailUsecase | Nuxt 注册、登录、设置与恢复页 | `backend/api/blog/v1/auth.proto`、`internal/conf/conf.proto` | `make api/config`；token事务、SMTP、HTTP与真实BFF fixture | v14扩展User状态与目的token表；旧账号不加verified gate，密码改变以credential version撤销旧恢复能力 |

详情见 [账号邮箱验证与密码恢复](ACCOUNT-EMAIL.md)。

| CollectionService | HTTP/gRPC | backend CollectionService | Nuxt 收藏页与文章详情 | `backend/api/blog/v1/collection.proto` | `make api`；repo/权限/HTTP/MySQL与真实BFF fixture | v15扩展表；身份仅取claims，公开文章动态过滤，重复保存和取消幂等 |

详情见 [跨设备收藏](COLLECTIONS.md)。

## 小程序身份

微信登录、绑定、v16身份表及回填命令见[小程序身份契约](WECHAT-IDENTITY.md)。公开请求形状不变，服务端按AppID隔离并显式启用，旧列保留。

## 生成物规则

- 机器使用的固定文件名保持稳定，不复制出 `final`、`new`、`latest` 等并行版本。
- 生成物写明来源、生成时间和版本/提交身份；无法验证来源时不伪造更新日期。
- 自动生成文件不手工修补；修改源定义后重新生成并检查差异。
- 历史快照进入归档并标注日期，不继续作为当前导入源。
- 示例只使用占位符或环境变量，不包含真实凭据、Cookie、Token 或签名 URL。

## 变更流程

契约变更先使用 [CHANGE-CHECKLIST.md](CHANGE-CHECKLIST.md) 判断兼容性。新增契约从 [CONTRACT-TEMPLATE.md](CONTRACT-TEMPLATE.md) 建立；涉及数据 Schema 或回填时同时使用 [MIGRATION-TEMPLATE.md](MIGRATION-TEMPLATE.md)。

## 搜索与问答

[结果状态与兼容策略](SEARCH-CHAT-OUTCOMES.md)：增量outcome、降级语义与消费者回归入口。
