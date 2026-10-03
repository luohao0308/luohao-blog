# Design

## Source of truth

- Status: Active draft
- Last refreshed: 2026-09-30
- Primary product surfaces: Nuxt 前台首页、文章列表与详情、作品集、关于页、全局导航与页脚
- Evidence reviewed:
  - 现有 Nuxt 页面与组件：`frontend/app/layouts/default.vue`、`frontend/app/pages/index.vue`、`frontend/app/pages/archives.vue`、`frontend/app/pages/posts/`、`frontend/app/components/ArticleCard.vue`
  - 改版计划：[docs/plans/SITE-REVAMP.md](docs/plans/SITE-REVAMP.md)
  - 用户提供的原型目录：`/Users/luohao/Downloads/Revamp Personal Blog Design/`
  - 原型路由：`/`、`/writing`、`/writing/:slug`、`/work`、`/work/:slug`、`/about`
- Implementation contract: 本文件约束视觉语言和接入边界；路由与后端契约以当前 Nuxt 代码和 proto 为准。

## Brand

- Personality: 安静、克制、清楚，有个人工作痕迹；偏编辑感而不是营销站。
- Trust signals: 真实文章、明确日期和标签、项目的技术栈与角色、可访问的 GitHub/邮箱。
- Avoid: 大面积渐变、装饰性玻璃拟态、过重阴影、虚构文章或项目数据、将原型中的示例身份直接带入产品。

## Product goals

- Goals:
  - 让访客在首屏理解作者是谁、关注什么、最近在做什么。
  - 让文章和项目成为可扫描、可继续阅读的内容入口。
  - 保留 SSR、真实 API 数据和暗色模式的现有能力。
- Non-goals:
  - 不把 Figma Make 原型整体迁移为独立 Vite 应用。
  - 不在 P2/P3 提前实现管理后台、评论、AI 侧栏或后端 projects 模块。
- Success signals: 首页、文章、作品集、关于四类入口可达；真实文章渲染；移动端可用；`pnpm lint && pnpm typecheck && pnpm build` 全绿。

## Users, actors and jobs

| Actor | Need | Success signal |
|---|---|---|
| 技术读者 | 快速判断作者方向并找到相关文章 | 首屏定位清楚，文章可按标签和时间扫描 |
| 潜在合作方 | 了解作者能力、项目经验和联系方式 | 关于页提供简介、技能、经历和联系入口 |
| 作者本人 | 持续发布内容并展示项目 | 文章来自 API，项目先由类型化占位数据驱动 |

## Information and system flow

- 全局导航映射：`/` 首页、`/posts` 文章、`/projects` 作品集、`/archives` 归档、`/about` 关于。
- 原型的 `/writing` 映射到现有 `/posts`；原型的 `/work` 映射到现有 `/projects`。原型英文路径和“林叙 Lin Xu”身份仅作参考，不直接复制。
- 首页：作者定位 → 当前状态/进行中 → 精选文章 → 精选项目 → 快捷入口。
- 文章列表：筛选/标签 → 卡片列表；详情：元信息 → 正文 → 上一篇/下一篇。
- 作品集：类型化占位数据 → 列表 → 详情；后端 projects 保留给 P4。

## Design principles

- 内容优先：标题、摘要、日期和标签比装饰更重要。
- 低噪声层级：使用大留白、细边框、轻底色和少量彩色表面建立层次。
- 真实数据优先：文章不使用原型静态示例替代 API；空状态要明确说明。
- 渐进增强：动效只辅助进入和 hover，`prefers-reduced-motion` 下必须可读、可操作。
- 复用现有 Tailwind 与 Nuxt 组件，不引入新的 UI 框架。

## Key decisions

| Decision | Choice | Reason/evidence | Consequence |
|---|---|---|---|
| 原型接入方式 | 提取视觉语言和页面结构，重写为 Nuxt 页面 | 当前项目已有 SSR/API/路由，原型是独立 Vite 静态应用 | 不复制原型的 `src/App.tsx` 或路由实现 |
| 文章图片 | 首期允许使用稳定的本地视觉资源或无图版卡片 | 当前 Article API 没有封面字段，不能伪造内容 | 后续有媒体契约后再接 cover 字段 |
| 色彩 | 白底/墨色文字为主，蓝灰、淡紫、薄荷、沙色作为小面积表面色 | 原型的层次感来自低饱和表面色 | 所有颜色同时提供 `dark:` 语义值 |
| 形状 | `rounded-lg` 以下的小圆角，细边框，克制阴影 | 符合原型的编辑感和仓库前端约束 | 避免卡片套卡片 |
| 主题 | 延续现有 `html.dark` + CSS variables + localStorage | P1 已完成并在计划中固定 | 新页面必须覆盖暗色、focus、空状态 |

## States and failure behavior

| State/failure | User/system behavior | Recovery |
|---|---|---|
| 文章加载中 | 保持布局稳定，显示简洁占位 | 请求完成后替换内容 |
| 无文章 | 显示“暂无文章/第一篇文章正在路上” | 提供文章入口，不渲染空卡片 |
| API 错误 | 页面显示可理解的失败提示 | 保留导航，允许重新进入或刷新 |
| 不存在的文章/项目 | 使用 Nuxt 404 | 提供返回文章/作品集入口 |
| 移动端导航 | 折叠为菜单或可横向扫描的入口 | 键盘和触摸均可操作 |

## Accessibility, security and privacy

- Accessibility: 目标 WCAG 2.1 AA；语义标题、`nav`/`main`/`article`、可见 focus、跳过导航链接。
- Security/privacy: 外链使用 `noopener`；文章 HTML 继续由受信任后端渲染；不在前端写入凭据。

## Implementation constraints

- Framework/styling: Nuxt 4 + Vue 3 + Tailwind CSS 4，保持当前 SSR 和 server API proxy。
- Data: 文章使用 `useArticleList`/`useArticleBySlug`；项目使用 `frontend/app/data/projects.ts`，直到 P4。
- Route mapping: 使用当前 `/posts`、`/posts/[slug]`、`/projects`、`/projects/[slug]`，不新增 `/writing` 或 `/work` 别名，除非 SEO 迁移另立任务。
- Performance: 首屏不加载大体积图库；图片若使用，必须有尺寸、`alt` 和 lazy loading。
- Test/screenshot expectations: 页面级 SSR 冒烟；前端 lint/typecheck/build；关键页面至少检查桌面和窄屏布局。

## Acceptance criteria

- [ ] 首页采用“作者定位 + 当前状态 + 精选内容”的原型结构，并接入真实文章与现有项目数据。
- [ ] `/posts` 卡片化展示文章，支持标签入口；详情页提供上下篇导航。
- [x] `/projects` 与 `/projects/[slug]` 延续原型的项目展示层级，数据来自类型化占位文件。
- [ ] `/about` 提供简介、技能/关注方向、经历和联系方式。
- [ ] 全部页面支持现有暗色模式、键盘 focus、加载/空/错误/404 状态。
- [ ] `pnpm lint && pnpm typecheck && pnpm build` 通过。

## Open questions

- [ ] 是否为文章引入封面图字段和媒体存储契约？影响文章卡片的图像版式，留给后端/内容模型变更时决定。
- [ ] 项目占位数据的真实项目名称、链接和截图由谁提供？在 P3 开工前补齐。
