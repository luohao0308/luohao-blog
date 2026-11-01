# 文章版本与修订历史

## 权威源与权限

`backend/api/blog/v1/article.proto` 是机器契约源。新管理员 API：

- `GET /v1/articles/{slug}/revisions?page_size=20&page_token=...`：元信息分页，不返回正文。
- `GET /v1/articles/{slug}/revisions/{version}`：不可变快照详情，Markdown 使用当前安全渲染器重渲染。
- `POST /v1/articles/{slug}/revisions/{version}/restore`，JSON body `{"expected_version":"2"}`：将服务端保存的历史内容恢复为新修订，返回新的 Article。

均仅 ADMIN；匿名/读者不得访问；已删除文章仍返回404，本轮不提供回收站或复活接口。

## 版本条件

Article.version 为 uint64，JSON用十进制字符串。创建固定为1。更新的 `expected_version` 在 query（原body仍为article），删除同样在 query；缺少或0返回400 ARTICLE_VERSION_REQUIRED，过期返回409 ARTICLE_VERSION_CONFLICT。不得保存前重读最新版本替代客户端基准，否则无法保护旧窗口。

数据库实际标量 CAS 成功后再在同事务写正文与关联。Ent带关联更新会预选ID，不能凭builder.Where外观证明原子版本判断；真实MySQL并发用例验证只有一个旧版本请求成功。

## 快照与恢复边界

- 迁移17新增version；18新增article_revisions，唯一(article_id,version)。保留原迁移1–16，无跳号、复用或删除。
- 创建、保存、分类解绑、软删均保留修订；正文/版本/标签分类/快照/公众号outbox事务一致，失败整体回滚。阅读/点赞计数不推进编辑版本。
- 接入前旧文章首次变更补baseline，作者未知，时间采用原正文更新时间；不虚构此前历史。
- 快照含标题、摘要、Markdown、标签/分类身份、封面和精选；操作人从认证context取得，不相信客户端作者字段。
- 恢复只复制服务端快照白名单；保持当前ID、slug、发布状态、首次发布时间和计数，生成action=restore的新版本；旧快照不覆盖。
- 历史分类按原ID及slug核验，不能把同slug重建的另一分类当成原分类；托管封面通过本地媒体存储检查，不由服务器任意抓取正文外链。失效分别返回409 ARTICLE_REVISION_CATEGORY_UNAVAILABLE / ARTICLE_REVISION_MEDIA_UNAVAILABLE。
- 已发布内容恢复后会更新公开正文；保持原公众号规则，不增加新的公众号发布意图。搜索索引在提交后best-effort更新，失败由搜索同步面板单独核验，不回滚正文历史。

## 后台行为

409保留输入及本地草稿，不自动刷新覆盖或自动重发。草稿保留base_version并隔离窗口；成功仅清对应owner/快照记录。脏稿、待恢复草稿、图片上传中不可直接恢复；恢复期间冻结输入，成功才同步Milkdown、基准版本及dirty状态。

差异只按纯文本显示；合计128KiB、每侧1000行、最多100000个矩阵单元，任一超限降级并排预览和完整Markdown下载。没有新依赖或无界diff计算。

## 交付与回退

旧无版本写请求会被明确拒绝，需要刷新新版后台；内部seed、content-release与直接smoke消费者都须传原基准版本。本轮开发和验证不授权生产迁移/部署。正式交付先备份DB/uploads并验证恢复，在维护窗口整体更新后端和后台；保留新增列/表和修订数据，回退应用时停用新写入口，不直接执行会丢历史的down脚本。
