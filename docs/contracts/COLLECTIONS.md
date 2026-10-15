# 跨设备收藏

机器契约以 `backend/api/blog/v1/collection.proto` 与迁移 `000015_collections` 为准。

## HTTP 契约

- `GET /v1/collections/list?page_size&page_token`：返回当前登录账号的公开文章收藏及 `next_page_token`。
- `POST /v1/collections/{slug}`：收藏一篇当前公开文章；重复请求幂等，不刷新 `saved_at`。
- `DELETE /v1/collections/{slug}`：取消当前账号的收藏；目标不存在、属于其他账号或文章已隐藏时仍幂等成功。

请求中没有 `user_id`。服务端只从已验证 JWT claims 获取账号身份，ADMIN 与 READER 遵守同一公开文章边界。未知、草稿和软删除文章在收藏时统一返回 404。

列表在 SQL 层先同时过滤当前 `user_id` 和 `Article.status=PUBLISHED`，再应用 offset/limit，避免隐藏条目占用页位或泄露标题、正文。固定顺序为 `saved_at desc, article_slug asc, collection.id asc`；`article_slug` 是收藏时写入的不可变排序键，文章 slug 本身不可修改。

收藏表使用 `(user_id,article_id)` 唯一约束。并发或重复收藏只保留首条记录，返回原始 `saved_at`。文章下架或软删除不会硬删收藏行；重新发布后收藏可再次出现在列表。用户或文章发生真实硬删除时，由外键级联清理。

游客继续使用浏览器本地收藏。登录用户的本地导入由前端逐条调用保存接口，并且只在单条得到服务端成功确认后删除对应本地记录；未成功条目保留以便重试。
