# 个人站前台改版实施计划

_状态：approved | 更新：2026-09-30 | 关联任务：T-005 | 关联设计：站点 sitemap（对话确认，2026-09-30）_

## 1. 目标、成功标准与停止条件

- 目标结果：博客升级为完整个人站——门面首页、文章区、作品集、关于页（联系方式并入）、暗色模式；站点地图 = 2026-09-30 对话确认的 12 页清单。
- 可验收成功标准：导航五入口（首页/文章/作品集/关于）全部可达；暗色模式切换且持久化、SSR 无闪烁；作品集页面以类型化占位数据渲染；lint/typecheck/build 全绿。
- 完成后停止条件：不做管理后台（M2/S4）、不做后端 projects 模块（P4 切片）、不做评论与 AI 侧栏（M3/M4）。

## 2. 范围与非范围

### 范围

- 全局：导航栏（五入口 + 主题切换）、页脚、暗色模式（CSS 变量 + class 策略 + localStorage 持久化 + 防 SSR 闪烁）
- 首页门面：hero（名字/定位/社交链接）、精选文章（真数据）、精选项目（占位）、快捷入口
- 文章区升级：列表页卡片化、详情页上下篇（TOC 留待后续）
- 作品集：列表 + 详情（占位数据文件驱动）
- 关于页：介绍/技能栈/时间线/联系方式

### 非范围

- 后端 projects 模块（P4）、管理后台全部页面（M2/S4）、评论（M3）、AI 侧栏（M4）、i18n

## 3. 当前证据基线

- 代码/配置：main `52a2fb7`；公开页为 M1 极简版（index/archives/about 占位、posts 列表+详情）
- 测试/CI：全绿流水线稳定
- Unknown：无

## 4. 规模判定与用户确认

- 规模：large
- 触发信号：跨多页面 + 三个有序切片
- 确认状态：approved
- 用户确认时间或消息指针：2026-09-30 用户确认 sitemap 并指示「做」，决策：联系方式并入关于页、暗色模式采用 CSS 变量自动适配方案

| 切片 | 目标结果 | 修改范围 | 依赖 | 验收方式 | 回退点 | 状态 |
|---|---|---|---|---|---|---|
| P1 | 站点骨架 + 暗色模式 + 首页门面 + 关于页 | layout、useTheme、index、about、projects 占位数据 | M1 | build + SSR 冒烟 | main `52a2fb7` | completed + **merged**（PR #7，merge `f238b4b`） |
| P2 | 文章区升级 | posts 列表卡片化、详情上下篇、标签页 /tags/[tag] | P1 | lint/typecheck/build + SSR/浏览器冒烟 | P1 合并点 | completed（本地，待交付） |
| P3 | 作品集页面 | projects 列表 + 详情（占位数据） | P1 | lint/typecheck/build + SSR/浏览器冒烟 | P1 合并点 | completed（本地，待交付） |
| P4 | 后端 projects 模块 | proto + 表 + CRUD + admin 页接线（与 M2/S4 合流） | P3 + M2/S3 | 契约测试 + 冒烟 | P3 合并点 | pending |

## 5. 原则与决策

| 决策 | 选择 | 理由 | 代价 |
|---|---|---|---|
| 联系方式 | 并入关于页 | 个人站惯例，避免空页面 | 无 |
| 暗色模式 | class 策略 + CSS 变量 + 手动切换（light/dark/system），localStorage 持久化，head 内联脚本防闪烁 | Tailwind v4 官方做法；SSR 首帧无闪烁 | 所有页面需双套色值意识 |
| 前台技术 | 保持纯 Tailwind，不引入 Naive UI（Naive UI 只进 M2 管理后台） | 前台轻量、控制力强 | 无 |
| 作品集数据 | `app/data/projects.ts` 类型化占位 | 页面先于后端可用；P4 替换为 API | 占位内容需替换 |

## 6. 实施切片

### P1：站点骨架 + 暗色模式 + 首页门面 + 关于页

- 状态：pending
- 修改范围：`app/assets/css/main.css`（dark variant）、`app/composables/useTheme.ts`、`app/components/ThemeToggle.vue`、`app/layouts/default.vue`、`app/pages/index.vue`、`app/pages/about.vue`、`app/data/projects.ts`
- 切片验收：lint/typecheck/build 全绿；暗色切换持久化；首页/关于 SSR 正常
- 回退点：main `52a2fb7`

### P2：文章区升级

- 状态：completed（本地，2026-09-30）
- 修改范围：`frontend/app/pages/posts/index.vue`、`frontend/app/pages/posts/[slug].vue`、`frontend/app/pages/tags/[tag].vue`、`frontend/app/components/ArticleCard.vue`、`frontend/app/components/ArticleCollection.vue`、`frontend/app/composables/useArticles.ts`、文章样式
- 结果：卡片列表、搜索、标签入口、已发布过滤、分页聚合、详情上下篇、API/404/空/错误状态、暗色 Markdown 样式
- 验证：`pnpm lint`、`pnpm typecheck`、`pnpm build` 通过；真实 API 的 Nuxt SSR 冒烟覆盖 `/posts`、`/tags/go`、文章详情和 404；独立 fixture + Playwright/本机 Chrome 覆盖分页、已发布过滤、timestamp 对象、搜索、标签、未知标签、404/503、上下篇客户端跳转、390px 无溢出和无浏览器异常。截图：`.omx/artifacts/p2/`（本机）。
- 限制：当前公开列表 API 不支持 tag/status 查询，标签与相邻文章暂聚合全量分页；文章规模变大时应补后端查询契约。首页/全局导航的原型重设计属于后续视觉基础切片，本次仅 P2。

### P3：作品集页面

- 状态：completed（本地，2026-09-30）
- 修改范围：`frontend/app/pages/projects/index.vue`、`frontend/app/pages/projects/[slug].vue`、`frontend/app/components/ProjectCard.vue`
- 结果：项目列表、技术栈筛选、项目详情、源码/在线体验外链、空状态和 404；继续使用 `app/data/projects.ts` 占位数据，不接后端 projects。
- 验证：`pnpm lint`、`pnpm typecheck`、`pnpm build` 通过；Playwright/本机 Chrome 覆盖列表、筛选、详情返回、未知项目 404、390px 无溢出、暗色模式和页面异常检查。截图：`.omx/artifacts/p3-projects-mobile-dark.png`（本机）。

（P4 实施段在开工时补充）

## 7. 偏移控制

- 当前允许修改的切片范围：P1 列出的文件
- 需要重新确认的变化：范围、顺序、接口、迁移或风险发生实质变化
- 不需要重新确认的变化：切片内部的一般实现细节调整

## 8. 契约、迁移与发布

- 兼容策略：无后端变更（P4 除外）
- 发布顺序：P1→P2→P3 独立 PR；P4 待 M2/S3 后
- 回滚/恢复：revert 对应 PR

## 9. 测试与验证矩阵

| 层级 | 要证明的声明/场景 | Test/Eval/Check | 命令/入口 | 通过条件 |
|---|---|---|---|---|
| 构建 | lint/typecheck/build | Check | pnpm lint/typecheck/build | 全绿 |
| E2E/冒烟 | 各页 SSR 渲染、暗色 class 生效、404 | 断言 | preview + curl | 断言通过 |

## 10. 风险与缓解

| 风险 | 概率/影响 | 早期信号 | 缓解/恢复 |
|---|---|---|---|
| Tailwind v4 dark variant 配置差异 | 中/低 | dark: 类不生效 | 用 @custom-variant 官方语法，冒烟时断言 class |
| SSR 主题闪烁 | 中/低 | 首帧白闪 | head 内联脚本提前挂 .dark class |

## 11. 交付状态与 PR 证据

- 当前状态：P1 **merged**（PR #7，merge `f238b4b`，2026-09-30）；P2/P3 **merged**（PR #10，merge `91e2f5f`，2026-09-30）
- PR 编号或链接：逐切片建立
- source ref / target ref：feat/site-p* → main
- P2/P3 PR：[#10](https://github.com/luohao0308/luohao-blog/pull/10)，head `4721d2bb33da0742c9fa9b65528334c493d2a7ec`，CI run `36745546102` 两 job pass；squash merge `91e2f5ff0feee55fe6c21f6e873ab5cb416590e3`（2026-09-30）。独立 Review 门禁已按用户要求从项目与 GitHub 规则中移除。

## 12. 文档同步

- [x] `TASKS.md` / 上下文
- [x] `PROJECT-SUMMARY.md`
- [x] 站点地图（本文件 §2 即权威）

## 13. 完成定义

- [x] 大型计划已获得用户确认并记录切片版本。
- [x] 所有切片验收通过，且过程状态按顺序更新。
- [x] 适用 lint、类型检查、构建和 CI 通过。
- [x] 项目摘要、任务板与交接文档已同步；本切片未改变后端契约。
- [x] 最终证据、SHA/产物身份和剩余风险已记录。
- [x] 如已进入远端交付，PR 和 CI 证据完整；merge 经 fail-closed 门禁授权。
