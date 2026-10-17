# Design

## Source of truth

- Status: Active
- Updated: 2026-10-18
- Surfaces: Nuxt公开阅读端、账号菜单、RAG问答、Naive UI管理端、Milkdown文章编辑器。
- Evidence: 当前main bd762c2的布局/组件/文章工具与proto；[已批准优化计划](docs/plans/UI-REFINEMENT-2026-10.md)。
- Reference: [open-claude-ui固定版本文档](https://github.com/Caho1/open-claude-ui/tree/02d021fceaf2a907f9e63a8125621bdda1b85d60/docs)，v1八篇、v2六章与合并版差异全文阅读并核对相关示例。参考设计原则，行为以本项目代码和测试为准，不以第三方逆向叙述代替官方事实。
- Visual proposal: 用户未采用暖纸色视觉稿；[原视觉计划](docs/plans/archive/VISUAL-REDESIGN-2026-10.md)已标为superseded。用户已确认白底融合版；当前实现以[首页动效计划](docs/plans/HOME-MOTION-2026-10.md)为准，Active交互规范继续有效。
- Visual reference audit: 2026-10-17重新核验，最新固定版本为[ddf3aea](https://github.com/Caho1/open-claude-ui/tree/ddf3aea1711db62fd6d24343bbab500a5ae69f64)，较旧版本增加6次提交；新设计依据v1 §1.3–1.5、cds-theme.css与SegmentedControl示例。新版修订表面阶梯、强调色角色与分段控件内边距，不能继续把旧数值称为最新规范。

## Brand

安静、克制、清楚，有真实个人工程实践痕迹。保留蓝灰主色、薄荷辅助色与中性表面；以真实文章、日期、项目和作者身份建立信任。避免大片渐变、厚重投影、装饰性玻璃、虚构内容或照搬Claude品牌。

## Product goals

- 阅读、搜索和参考文章之间可自然跳转；单轮问答的等待、降级与失败可理解、可恢复。
- 作者编辑内容可保留，保存结果可信；全站明暗、键盘、触摸均可用。
- 非目标：新UI框架/依赖、流式或多轮会话、模型选择、音频、MCP/Artifacts沙箱、离线自动重发、自动发布、持久草稿。

## Personas and jobs

| 用户 | 任务 | 完成信号 |
|---|---|---|
| 技术读者 | 搜索、阅读、查看代码和架构图、问博客内容 | 命中结果可见，长文可定位，代码/图片操作可恢复 |
| 潜在合作方 | 查看真实项目、作者介绍和联系方式 | 信息清晰、入口可访问 |
| 作者 | 在现有管理端编辑/保存文章 | 初始化/保存失败保留输入，未保存离开有提醒 |

## Information architecture

保持首页、/posts、/tags、/categories、/archives、/projects、/about、账号页面和现有/admin路由。文章详情保留元信息、正文/目录、问答、评论、相关文章和上下篇。作品和关于已使用真实后端数据；不恢复旧原型占位数据和/writing、/work路由。

## Design principles

内容优先；使用留白、轻底色和细分隔建立层级；减少嵌套边框但保留可辨识控件和焦点。SSR原文先可读，客户端增强。只有真实服务端信号才展示阶段/成功；失败可就地恢复。

## Visual language

- Color: main.css拥有surface-page/panel/input/overlay、text-primary/muted、border-default/strong、accent/hover/on-accent、status与focus语义变量，html.dark配对覆盖。蓝灰#3c5d85与暗色浅蓝延续；Naive主题映射同一组变量。
- Typography: 系统中文字体与等宽代码字体；阅读端舒适行距，长词安全换行；日期/计数tabular-nums。不给系统字体机械施加专有可变轴。
- Spacing: 小控件紧凑，正文大留白，后台表格密度独立于阅读端；不新增密度开关。
- Shape/elevation: 标准控件约8px、代码约12px；公开内容卡片与账号面板按[卡片规范](docs/plans/BENTO-CARDS-2026-10.md)使用24px圆角、三层柔影、悬浮上移4px与指针追光。管理端保持既有控件规范。
- Motion: 短颜色/opacity/transform反馈；统一时长/缓动。减少动态偏好下移除非必要动画和平滑滚动，不以CSS本身保证帧率。
- Imagery/iconography: 真实封面/媒体、alt与尺寸；使用现有图标。正文不套滚动渐隐遮罩。

## Components

复用ArticleCard/ProjectCard/ThemeToggle/UserMenu/ArticleCollection、Naive UI和Milkdown。账号菜单采用disclosure与自然Tab链接；新增阅读增强组件只拥有目录/复制/图片交互，不引入第二套正文渲染。问答回复保持纯文本，参考卡片来源于真实API。共享样式由main.css管理，组件负责业务状态。

## Accessibility

目标WCAG 2.1 AA；语义标题/nav/main/article、跳过导航、可见focus覆盖textarea/select/contenteditable。主要触控目标44px为产品舒适度目标，不误引为2.1 AA要求。Escape关闭浮层并归还焦点，模态预览限制焦点在弹窗内，Tab能退出普通导航组。状态除颜色外有文字，错误可播报，IME合成中的回车/Escape不被误消费。

## Responsive behavior

保持现有断点，卡片与工具栏可按容器实际宽度折行，min-width:0和安全断词；320px不出现页面横向溢出。长代码/表格内部滚动，手机目录折叠，桌面目录旁栏；关键操作不只依赖hover。浏览器新特性仅渐进增强。

## Interaction states

- 搜索：保留服务端相关性顺序；只有本地降级才字面过滤；加载不清空已有列表，空结果/不可用区分。
- 问答：每次问题独立，保留请求快照，等待/失败/返回结果明确；不凭HTTP200或文案猜结果，不重复添加同一重试问题。参考文章不是逐句证据。先手动重试，不自动重发计费请求。
- 阅读：复制Promise成功才显示成功；拒绝可恢复且保留原始空白。图片适屏、Escape关闭、焦点归还；禁用JS正文仍可读。
- 编辑：加载/失败重试、编辑/保存中/失败/成功；初始化重试和保存失败保留正文；有改动离开提醒，保存成功重置dirty。

## Content voice

简体中文、短句、可操作；“参考文章”“每次问题独立回答”。不伪造“正在检索/推理中”等后台阶段；不暴露实现细节、权限凭据或模型内部推理。

## Implementation constraints

Nuxt4/Vue3/Tailwind4，Naive UI后台，Milkdown编辑器；保留BFF/SSR和后端安全HTML/高亮。proto为契约源，新增结果字段向后兼容并生成代码，同步Web/小程序。无新依赖、无DB迁移、无远端/生产操作。测试先行，逐片lint/typecheck/build与浏览器检查；契约变化再加Go与小程序回归。截图为本地证据，不提交。

## Open questions

- 本地持久草稿恢复、流式/多轮、图片高级缩放：本轮不做，未来需独立范围与验收。
- 真实移动设备上的键盘与触控：本轮自动浏览器模拟不等于真机验收，最终报告保留此边界。

## Approved home motion integration

- 最新修正：用户明确拒绝“框内播完再跳成真实网站”。开场直接作用于真实SSR首页的导航、介绍、统计、文章、项目与页脚，不创建演示画布、仿浏览器顶栏、克隆文档或结束替换。
- 保留原分层开场54°→36°→29°的空间变化、2900ms、65ms错峰和连续缓动；位置以真实响应式布局为准，92%以后transform:none、opacity:1，取消效果后仍是同一DOM、同一位置。多余真实区块共用第5层节奏，最长3160ms。
- 文章扇开作用于前三个真实文章卡片，1900ms、530+i*65ms延迟，按真实网格测量聚合中心；最后回到原网格，保留原文本与操作，不叠加另一套卡片副本。
- 初次、支持动效且未见过的首页通过head短暂隐藏真实页面，动画就绪后同帧显现；1500ms安全超时与bfcache恢复保证失败不永久遮挡。无JS/无WAAPI/减少动态直接显示页面；同一标签页仅播一次，Skip/Escape/交互/路由/resize/偏好/隐藏/异常可退出。
- 默认浅色，head和mounted初始化保持一致。保留用户显式dark/system选择，localStorage拒绝时安全浅色回退。真实元素同主题播放，禁止白色广告画布与暗色网页之间切换。
- 验收使用真实DOM身份、结束前后边界与像素一致性、首屏主题与降级检查。冻结原版仅作为历史轨迹参考；旧广告画布的0.03%像素结果不再用于宣称当前全页效果。

## Article card expansion

2026-10-18用户追加：文章卡片点击按[卡片规范](docs/plans/BENTO-CARDS-2026-10.md)从原位展开为正文；保留真实路由、修饰键和嵌套标签操作，减少动态或不支持API时直接导航。项目卡片继续原导航。

## 关于页墨流

2026-10-18用户批准[关于页墨流预览](docs/plans/ABOUT-INK-2026-10.md)：全宽白底、蓝灰墨迹围绕个人字标持续流动，支持鼠标扰动、点击扩散与滚动收拢。此首屏是原有克制表面规范的明确例外；下方继续使用真实个人资料与既有卡片规范。文字独立于画布，减少动态、无WebGL及SSR仍可读；无新依赖、无生产操作。
