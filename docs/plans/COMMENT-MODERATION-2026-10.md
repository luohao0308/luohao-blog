# 评论审核与交互优化

_状态：completed | 更新：2026-10-11 | 关联任务：T-014_

## 目标与范围

提交时进行本地确定性风险检查，普通评论立即公开，可疑评论留在现有 PENDING 人工队列；同步修复评论可见性、分页和客户端反馈。成功标准分别对应后端回归、客户端逻辑测试和浏览器闭环。

不接收费 API、不增加依赖、不迁移或回填旧评论、不改变远端 Git 或生产环境。规则只承担轻量反垃圾，不能保证识别所有违规语义。未来可单独接入腾讯云 TMS。

## 确认与基线

- 规模：large；跨 backend/frontend/miniprogram，包含多个独立验收结果。
- 用户 2026-10-11 回复「可以 确认」，批准此前对话中的两个切片。
- 只读基线：全部新评论强制 PENDING；公开列表未检查文章公开状态；Web/后台忽略分页 token；小程序丢弃创建结果并误认为公开接口会带回待审评论。
- 基线验证：Go biz/service/data 测试通过，小程序逻辑测试通过（原待审 fixture 不符合公开接口）。
- 追踪：本机 T-014；远端 Issue 尚未创建，须按仓库权限独立授权。

## 切片与状态

| 切片 | 结果与范围 | 依赖 | 验收 | 回退 | 状态 |
|---|---|---|---|---|---|
| S1 | backend 自动审核策略、人工模式开关、文章公开状态检查、契约注释与配置 | 无 | 普通通过/可疑待审/限流异常待审/下架隐藏/身份与长度回归；构建与 lint | 配置切回 manual；保留旧待审数据 | completed |
| S2 | frontend/miniprogram 根据响应状态展示、分页、并发保护与长度提示；后台默认待审 | S1 | 两种状态反馈、加载更多、加载锁、小程序真实 fixture；lint/typecheck/build 与浏览器冒烟 | 独立回退客户端修改 | completed |

## 实施决定

- 使用既有 APPROVED/PENDING 和审核/删除接口，无新增 API 字段或数据库迁移。
- 默认 smart；manual 保留全部人工审核；未知模式按 manual 处理。
- 本地规则覆盖链接/联系方式、明显导流词和刷屏；自定义 review_keywords 可补充，正常技术讨论和批评不按情绪拦截。
- 保留 IP 预算并增加账号预算；Redis 异常时允许保存但进入待审，防止无频控直接公开。
- 公开接口只返回公开文章的已通过评论；作者刚提交的待审评论由本次创建结果在页面内展示，不宣称刷新持久化。
- 页面计数标注已加载数量，避免将分页数量当作总数；以返回 ID 去重。

## 验证与发布

先添加失败回归，再实现 S1；S1 验证通过后将其标记 completed，才开始 S2。最终检查差异、生成文件及契约，更新任务状态。

后端命令：go test ./...、go build ./...、go vet ./...、golangci-lint run；本机 shell 的 GOROOT 指向旧 SDK，运行时用 env -u GOROOT 与已有 Go 1.26 工具链，不修改用户配置。
前端命令：pnpm lint、pnpm typecheck、pnpm build。小程序：npm run typecheck、npm test。浏览器：本任务拥有的本地服务/测试 fixture，禁止复用生产凭据或写入生产。

交付状态：local_verified（本地完成；没有 push/PR/部署授权）。

## 验证记录

S1：新增失败回归先复现普通评论待审、空白内容入库和下架评论泄露；修复后 go test ./...、go build ./...、go vet ./... 全通过，golangci-lint run 为 0 issues；make config/api 和 go generate ./cmd/server 已刷新生成物。独立只读审查发现代码行注释被过宽的双斜杠规则误判，已收窄并补回归。


S2：frontend pnpm lint/typecheck/build 全通过；miniprogram npm run typecheck && npm test 全通过。新增失败回归复现并修复加载锁、emoji 头像首字与首屏刷新失败保留；补 deferred POST 跨账号隔离、待审转公开去重、失败保留草稿和 Unicode 边界检查。

后端增加真实 HTTP 契约回归：JWT + Casbin + services/usecases + SQLite + miniredis，覆盖访客拒绝、身份不可伪造、普通即时发布、可疑不公开、读者无审核权限、管理员批准、分页和下架隐藏。MySQL timestamp 秒级同值另以固定时间戳 + ID 排序回归覆盖。

浏览器在 Nuxt 生产构建产物 + 上述临时后端 fixture 完成：公开分页、reader 发布、创建后列表 GET 503 与首屏重试、私有待审预览/刷新消失、1000/1001 Unicode 输入、后台超过 50 条翻页、分页 GET 503 后页码保持/重试、批准后公开和测试数据清理。输出：`Comment browser PASS: public pagination, reader identity, immediate publication, private pending preview, admin pagination, approval, cleanup`。截图仅保存在本机临时 artifacts，不提交。

独立只读复核：修复代码注释误判、管理分页失败漂移、Web 首屏错误重试和小程序跨账号/emoji 问题后，reviewer 返回 APPROVE，未发现阻塞项。

### 变更文件分组

- 后端：`internal/biz/comment{,_moderation}.go` 及回归，`internal/data/comment{,_test,_http_test}.go`，服务层构造参数回归；配置映射与 Wire provider。
- 契约/配置：`api/blog/v1/comment.proto`、`internal/conf/conf.proto` 及重新生成的 pb/HTTP/gRPC/Wire/OpenAPI；开发和生产配置模板显式 `smart`。
- Web：`app/components/CommentSection.vue`、`app/pages/admin/comments.vue`。
- 小程序：post TS/WXML/WXSS、评论 API 返回类型、`scripts/verify.cjs`。
- 验证/文档：评论专项浏览器脚本、既有 t15 的可疑内容 fixture、复现 README、任务板和项目摘要。

### 限制与回退

- 未安装新依赖，无云审核费用、无数据库迁移、无历史评论批量批准。
- 本地规则可能误判外链或漏掉语义/变体内容；`review_keywords` 可补充，管理员仍可审核/删除，配置 `manual` 可恢复全部待审。
- 待审预览只保留当前页面内存；没有私有评论查询接口，刷新后不展示此预览。
- 未验证真实 MySQL 完整部署栈、微信 DevTools/真机及线上新版本；Docker 当前未运行，浏览器使用独立 SQLite/Redis fixture。生产更新需另行授权，不能由本次实施确认推导。
