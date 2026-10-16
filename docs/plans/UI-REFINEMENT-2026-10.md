# 博客视觉与交互优化实施计划

状态：completed（本地验收） | 日期：2026-10-16 | 任务：T-018 | 设计：[DESIGN.md](../../DESIGN.md)

## 目标与边界

按用户在本会话全文阅读后的修订方案及“确认”实施 S1–S4。借鉴 open-claude-ui 固定提交 `02d021fceaf2a907f9e63a8125621bdda1b85d60` 的设计层级、输入保护、状态恢复与媒体交互，不复制 React/Framer Motion 体系；保持蓝灰/薄荷品牌、SSR 与现有渲染安全边界。

大型计划已批准；一次只推进一个切片。只在本地隔离工作树实施验证。无新依赖、无迁移、无远端 Git、无部署、无真实用户写入。Issue 尚未创建：远端 Issue 操作未授权，不阻塞本地实现。

## 证据基线与现状校正

- 基线 origin/main `bd762c2`，临时分支 `codex/blog-ui-refinement`；原工作树及其未提交改动保留。
- 最新主线已有文章摘要、封面、目录、媒体上传和小程序图片预览；S3复用目录与图片地址处理，不重复实现。
- 搜索前端再次字面过滤服务端结果；聊天为单轮整包纯文本，检索异常可被吞并为空结果、LLM失败为HTTP200保留相关文章。
- 现有真实命令：frontend pnpm lint/typecheck/build；backend go test/build/vet、golangci-lint；miniprogram pnpm typecheck/test。浏览器复用已有 Playwright，无生产访问。
- 参考文档与示例有不一致：离线队列、焦点陷阱和复制反馈等须按真实行为验收，不能以示例展示替代测试。

## 切片与验收

| 切片 | 目标/范围 | 依赖 | 可执行验收 | 回退点 | 状态 |
|---|---|---|---|---|---|
| S1 | 语义tokens、常用控件、前后台布局、键盘/触控/减少动态 | 无 | foundations浏览器回归；frontend lint/typecheck/build | 独立恢复样式/组件，不改数据 | completed |
| S2 | 搜索结果保留与降级；问答生命周期及兼容结果状态；Web/小程序消费者 | S1 | 搜索语义/正文命中、中文选词、重试去重、晚返回、200降级；Go契约测试、小程序测试、浏览器回归 | 增量字段，旧客户端兼容；前端独立恢复 | completed |
| S3 | 现有目录移动端折叠、代码复制、可访问图片预览 | S2 | 重复标题、SSR、剪贴板拒绝、长代码、Escape/焦点恢复/窄屏 | 移除增强，保留SSR正文 | completed |
| S4 | 编辑器初始化恢复、保存状态、未保存离开提醒、暗色 | S3 | 初始化失败/重试、保存失败保留正文、站内/关闭离开、暗色浏览器回归 | 保留现有编辑保存接口 | completed |

## 设计决策

- 颜色、层级、状态、焦点、圆角和动效统一语义命名；阅读端舒适，后台可紧凑，无密度开关。
- 主控件触控区域目标44px；无障碍自然Tab流，不为普通导航使用不完整的menu语义。
- 问答保留请求快照，手动重试；不做自动离线重发、不伪造进度/思考、不增加流式/多轮/模型选择。
- citations标注“参考文章”，不宣称逐句验证；显式结果状态避免用HTTP200或返回文案推测生成成功。
- 图片预览和代码复制渐进增强；复制成功基于Promise，原始空白保留；继续使用后端安全渲染与高亮。
- S4首轮不增加持久化草稿：该项在方案中为后续可选范围；保留输入、失败恢复与离开提醒先交付。

## 验证与完成标准

先建立最小失败回归再修改。每个切片记录失败基线和通过证据后推进下一片。各阶段使用本任务本地服务，最终停止自建进程；不操作其他服务。截图仅本地output，不提交二进制证据。最终完成条件为四片通过、适用lint/typecheck/build/test及独立代码复核通过；未执行生产/真机检查必须明确说明。

## 执行证据

- S1：完成。先复现skip link缺失（0 != 1），再通过完整前后台foundations浏览器回归（skip/disclosure/主题持久化/真实颜色/焦点/reduce/320px），并以原始SSR HTML确认n-button/n-base-icon/n-data-table样式首屏存在；修复旧module→Nuxt4样式收集兼容，截图目检通过。pnpm lint/typecheck/build通过，build日志/tmp/blog-ui-s1-build.log。
- S2：完成。基线搜索API返回非字面文章但前台链接不可见；修复后dev与生产产物浏览器回归通过：语义命中、晚返回/清空、本地降级、IME、500码点、原位重试/草稿、结果状态、复制成功/拒绝、旧响应/空回答、320px暗色。pnpm lint/typecheck/build通过（/tmp/blog-ui-s2-build.log）；Go test/build/golangci-lint与mini typecheck/test通过，proto/OpenAPI生成及HTTP数字契约同步。
- S3：完成。基线复制按钮0 != 1；复用既有目录，dev/生产产物reading回归通过：SSR正文/图片映射、320px目录真实位置、重复标题、复制空白/拒绝、模态焦点/Escape/卸载与showModal缺失/抛错降级。真实Go高亮golden贯通安全语言元数据，旧HTML继续通用“代码”，不回填。Go渲染/biz/service测试与lint、前端6条单元测试及lint/typecheck/build通过；日志/tmp/blog-ui-s3-build.log，截图output/ui-refinement/s3。
- S4：完成。基线缺persistent save state；编辑器初始化/失败Markdown退路可编辑保存、重试、暗色、保存失败/成功、旧响应与新正文dirty、同slug切换、创建冻结/晚返回/成功导航通过。独立审查发现200ms正文同步窗口后改同步ProseMirror事务，移除旧防抖写回，same-task立即保存/离开/beforeunload回归通过。Milkdown无title图的null caption已本地Remark边界规范化，初始/插图/输入/undo redo/保存/重载及封面/精选媒体回归全过；不改变URL或HTML信任边界。后台身份与原生复选框SSR结构稳定，最终editor零运行错误/零水合错误。

### 最终本地证据

- 前端：6条node单元测试、pnpm lint/typecheck/build通过；构建日志/tmp/blog-ui-final-frontend.log。
- 后端：go test ./...、go vet ./...、go build ./...、golangci-lint通过，lint 0 issues；增量proto/OpenAPI与真实渲染golden一致。
- 小程序：npm run typecheck、npm test通过；未改AppID/配置或上传发布。
- 最终生产产物六套浏览器全部退出0：ui_foundations、ui_search_chat、ui_reading、ui_editor、article_experience、account_avatar。所有模型/账号/媒体均为本地fixture，无真实服务写入。
- 独立代码复核APPROVE，已发现的问题全部闭环；git diff --check、关键Markdown链接检查通过。
- 本任务13000前端和13001 fixture已停止，截图仅保留在本地output/ui-refinement/。

### 验证边界

未运行远端CI、真实LLM集成、小程序真机或生产部署；浏览器移动视口不等于真机。旧存量HTML无语言元数据继续通用工具栏，不做回填。Crepe图块alt的比例规范化是库既有行为；以图节点/URL和保存重载验收。

## 交付

当前状态：local_verified。本地提交由Git记录追踪。分支codex/blog-ui-refinement，base bd762c2；准确本地提交以Git记录和本会话交付为准。PR/CI/远端合并/上线尚未执行，也不继承其他任务授权。
