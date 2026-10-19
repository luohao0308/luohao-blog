# 阅读布局、搜索与目录交互

状态：ready_for_delivery（用户已授权上线，本地验证完成，尚未远端交付）。2026-10-19 用户逐轮预览后的当前方案。

## 当前范围

- 文章与作品列表同宽；文章与作品详情实际内容列均860px，手机自适应。
- 两类列表标题旁放大镜点击后，输入框在图标左侧页面内动画展开并自动聚焦，推动图标向右，下方展开有效分类/标签或技术栈。鼠标打开和自动聚焦无高亮外圈，键盘打开/Tab焦点保留focus-visible指示；输入使用text+searchbox语义，不显示原生清除叉，取消独立关闭叉，再点放大镜或Esc收起。默认收起直接显示内容；选中条件可见且可清除。无独立弹窗，不创建分类数据。
- 目录不参与正文分栏。1360px及以上固定在正文右侧；窄屏固定入口打开原生popover，选标题后关闭。原生hash锚点平滑上下滚动，scroll-margin避开动态页眉；返回保留历史，减少动态效果时立即定位。
- 目录不使用ui-panel的卡片focus-within/hover外圈，保留普通边框和链接focus-visible键盘指示。
- 首页移除WechatFollow，关于页≥1360px固定于正文右侧留白，窄屏固定微信入口打开原生popover，底部不重复放置；原有公众号/微信号及复制交互不变。现有组件没有二维码，未新增。
- 保留文章/作品卡片原生750ms展开。Chrome在5200ms详情响应下会跳过原生DOM更新过渡，真实详情就绪后补同起点裁剪动画。隐藏页及减少动态效果不补动画。

## 验证

本地Chrome：1440/1366px正文860px且目录间距24px；1024/375px滚动中入口可用，无横溢。向下scrollY 0→108→690→846，向上846→507→431→427；标题停在页眉底部下约20px。浏览器返回恢复前一hash与846px位置。手机弹层关闭后连续滚动0→531→1080→1284，减少动态效果时100ms内立即定位。鼠标目录容器outline none，Tab聚焦链接focus-visible solid。

首页微信标题0处、关于页1处，1440/375px无横溢；复制按钮回显成功。scripts/e2e/wechat_sync_e2e.cjs已同步入口并通过node --check；含管理员写操作的整套脚本未执行，公开流程由浏览器直接验证。

动画工具与composable回归12项通过，含超时、隐藏页、动态减少动画设置和清理。Chrome真实延迟5200ms验证补750ms动画，正常文章/作品原生动画存在，减少动态效果作品跳转动画调用为0。

两类列表在1440px均为896px容器；两类详情均860px，375px均327px。两列表1440/375/320px搜索展开自动聚焦，无横溢且无dialog/popover。共享InlineSearch提供220ms宽度/180ms淡入，减少动态效果时关闭过渡。文章搜索Ent/标签Ent匹配1篇，清除恢复4篇；作品搜索luohao/技术栈Go匹配1项，清除恢复3项；关闭后条件仍可独立清除。没有分类数据时仅显示现有标签。

定向ESLint、typecheck通过；动画/导航、首页动效、关于页动效39项回归通过。过滤固定fixture另有11项断言通过。node --check scripts/e2e/wechat_sync_e2e.cjs通过；diff --check通过。最终生产构建通过并重启本任务3017预览；生产构建实测无搜索弹窗、输入自动聚焦、目录smooth且无点击外圈、首页微信0处/关于页1处。

截图：/tmp/blog-projects-inline-375.png；最终预览端口3017。

## 回退与边界

撤销本轮前端与E2E入口差异可恢复。没有新依赖或内容数据修改；上线请求前未部署、push或创建PR。原工作树已有用户文档改动未触碰。预览 http://127.0.0.1:3017/posts 。

## 搜索微调与关于悬浮验证

1440/375/320px两列表展开后input.right等于图标left，图标向右移动，关闭恢复图标态，无横溢；鼠标自动聚焦outline/box-shadow均none，键盘Enter打开outline solid；搜索Ent与清除条件正常。关于页1600/1920px滚到800px及底部2087px，微信侧栏top始终144px，位于正文右侧16px余量且屏幕内；手机375px入口/复制/关闭正常，无横溢。原组件不含二维码，未新增。

截图：/tmp/blog-search-right-icon-desktop.png、/tmp/blog-about-wechat-docked.png、/tmp/blog-about-wechat-mobile.png。

本轮最终pnpm lint/typecheck/build通过，生产构建3017复核：手机图标x采样123.75→193.19→311，input.right=icon.left=311；自动聚焦outline/box-shadow none，text+searchbox无原生search取消叉。1600px关于页底部侧栏仍固定可见且无横溢。截图：/tmp/blog-search-right-icon-mobile-final.png、/tmp/blog-about-wechat-docked-final.png。

## 常见桌面窗口直接显示微信卡

关于正文外壳由1024px收至908px（含左右24px内边距，实际阅读860px），微信卡缩为192px并将内部padding调整为16px。断点从1600px降为1360px。浏览器1366/1440px：中段scrollY800和底部2087时侧栏top均144px，正文容器与侧栏间距16px，侧栏在屏幕内且display block，无需点击；复制正常，无横溢。375px保留入口且无横溢。截图：/tmp/blog-about-wechat-1366-visible.png、/tmp/blog-about-wechat-1440-visible.png。

本轮lint/typecheck/build及diff --check通过。3017最终构建复核1366px：侧栏visible=true，点击入口visible=false，x1153/y144/width192，屏幕内无横溢。

## 微信复制点击外圈

WechatFollow移除bento-card通用交互类，保留原中性边框并使用静态shadow-sm；复制按钮沿用focus-visible。1366px鼠标与375px手机入口内点击复制：按钮/卡片outline均none，父卡片transform none且before/after无高光；复制成功提示正常。Tab返回复制按钮focus-visible=true、outline solid。截图 /tmp/blog-wechat-copy-no-ring-desktop.png。

复制外圈本轮lint/typecheck/build、diff --check通过，3017最终构建重启并实测复制：按钮与卡片outline none，成功提示正常。截图 /tmp/blog-wechat-copy-3017-verified.png。

## 作品卡片动效一致性

实际对照发现：两类卡片悬停均上移4px且阴影/指针/边框一致；文章列表进入时触发420ms、55ms错落的入场动画，作品列表未接入（记录为文章4次动画调用、作品0次）。作品列表复用useArticleReveal并加同一标记，现3张项目卡均调用同参数入场动画。ProjectCard封面由div改为原生详情NuxtLink（包含可访问名称），根卡片交互类与ArticleCard对齐。

开发浏览器六组点击：文章/作品标题、封面入口、空白区域均进入正确详情，原生article-expand与article-content过渡存在。公开文章均无封面，文章封面分支用本地浏览器临时详情锚点fixture验证点击路由，不修改内容数据。34项动效/展开/composable回归通过。最终3017的动效、慢请求、reduce-motion对照在本轮收尾验证。

作品动效最终3017验证：列表入场参数与文章均420ms/55ms错落；hover/按下均transform translateY(-4px)，cursor pointer、阴影与边框数值相同，移出transform none。作品标题/封面/空白点击均出现article-expand与共享article-content原生动画，正确落到/projects/luohao-blog并清理。作品API延迟5200ms触发750ms回退；reduce-motion下入场与点击动画调用均0；键盘Enter封面可导航；文章嵌套标签导航动画调用0。lint/typecheck/build、34项动效/导航回归和diff --check均通过，最终3017已重启。截图：/tmp/blog-article-hover-final.png、/tmp/blog-project-hover-final.png、/tmp/blog-project-expansion-middle-final.png、/tmp/blog-project-entry-middle-final.png。

## 正式交付准备

用户已明确授权本轮上线。范围仅本计划UI变更；使用feat/reading-layout-interactions交付分支，排除原工作树的其他文档。主工作区本机gitPolicy实际为manual + ai，未改持久权限策略；隔离工作树恢复同仓库原manifest用于guard检查。远端push/PR/merge仍需绑定准确版本的一次性授权并通过delivery_guard。

生产只读核验：前端healthy，当前镜像ghcr.io/luohao0308/luohao-blog-frontend@sha256:e1e96e32ef9d9d575ce0b56cb7aee5da1cabcd90c07d6e42c5f966c7749ee3fd；源码checkout仍7c6117e，但实际Compose有ui/home/layer/reference/live/bento覆盖链，不能只凭checkout判版本。最后bento覆盖绑定9185503583480dab0f4313f86f53f7596e12b246，与本轮基线一致。已有2026-10-19 03:10数据库备份。仅更新frontend，不改backend/数据库/Caddy；回退使用上述旧镜像及原完整覆盖链。发布前重新核实状态。
