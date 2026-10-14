# 文章媒体与展示元数据

机器契约权威源：`backend/api/blog/v1/media.proto`、`article.proto` 及其生成物；本文说明权限与消费约定，不替代 proto。

- `POST /v1/admin/media/images`：仅 ADMIN，JSON `data` 为 base64。接受实际字节不超过 2MiB 的 JPEG/PNG/WebP；共享 HTTP 4MiB 请求上限。响应 `url` 是本站相对 `/v1/assets/media/{32位小写hex}.{jpg|png|webp}`。不接任意文件/SVG，不接外部URL下载。
- `GET /v1/assets/media/{name}`：公开图片回读，返回正确图片Content-Type及不可变缓存。图片链接是公开资产地址；不返回用户资料、服务器路径或凭据。无效名称/丢失文件统一404。
- Web显示使用既有 `assetUrl` 映射到 `/api/v1/...`；正文保存本站相对路径，不写开发localhost或带认证参数的地址。图片保存在现有uploads卷的media子目录，由uploads备份覆盖，不自动删除历史文件。
- Article新增可选 `cover_url` 和 `featured`，字段tag分别15/16。封面仅允许服务生成的相对媒体路径或空。创建时可填；修改必须在 `update_mask` 中显式包含新字段，旧客户端省略时不清掉已有值。
- ListArticles bool过滤采用现有 AIP 语法 `featured`（或 `NOT featured`），不是 `featured=true`；支持featured排序。匿名/读者列表仍强制仅PUBLISHED，精选不放宽权限。
- 封面/精选编辑不自动重复生成公众号任务；公众号继续沿用既有默认永久封面。文章正文图片采用JPEG/PNG并尽量压缩至1MiB以内，以适配现有微信正文上传链；远端错误仍作为明确失败记录，不假报成功。
- 扩展迁移 `000012_article_media` 为已有文章提供空封面/非精选默认值。回退保留数据和图片，勿在生产直接运行down丢弃字段。

可执行检查：`python3 scripts/test_mysql_migrations.py --baseline 11 --target 12` 验证实际嵌入迁移、旧数据保留、默认值、mysqldump+gzip恢复与恢复后重升级（仅本任务隔离MySQL）。媒体权限/格式/字节回读及旧客户端更新语义由backend相应tests证明；前端阅读/上传browser回归另见scripts/e2e目录。本地验收不代表已上线。
