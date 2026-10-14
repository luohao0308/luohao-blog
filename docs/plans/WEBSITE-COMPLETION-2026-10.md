# 网站能力补齐（2026-10-15）

status: approved
confirmed: 2026-10-15 用户确认全部六片、按顺序实现
current_slice: S3 in_progress
S1-S2: local_verified
S4-S6: pending
mode: direct planning; no product edits before split confirmation
baseline: origin/main fade43abed5578236a340e7034eba0df63401aff

## 目标与授权边界

用户确认实现上一轮列出的5项缺口及体验增强。本提案包含看门狗/后台分页、文章目录/阅读时长/媒体/封面/精选、作品集和关于后台维护及页面完善、SEO、邮箱验证/密码找回、收藏跨设备同步。保留现有博客账号、文章、评论、公众号草稿同步及HTTPS行为。个人主体公众号继续自动草稿+手动发表，不尝试绕过接口权限。公安备案状态只是未知，不属于可由代码实现的事项。小程序正式发布不在本轮范围。

用户已授权目标实现，切片已按AGENTS.md“大型计划拆分与确认门”获得用户确认。本次确认允许本地实现/测试，不扩大到远端Git、生产更新、真实发信或迁移；这些按准确版本、环境与动作准备完成后办理既有交付权限。不得为了准备切片修改持久权限策略。不得读取或输出私密配置。

最新只读证据：main SHA如上；域名收尾工作树de1fd4b与main等树（此前交付证据），当前该工作树clean。原主工作树的TASKS/公众号计划/DOMAIN计划/output属于既有改动，不覆盖、不顺带提交。实施时复用适合的附加工作树或新建隔离工作树，从重新核实的main开始。

## 顺序切片

同时仅一个切片in_progress。每片先准备最小失败检查，再实现，验收后推进下一片；片内允许不冲突的原生子代理并行实现/审查。不创建OMX tmux任务。

### S1 稳定性与后台分页

- 结果：看门狗探测真实backend健康路径，正常服务不误重启；文章管理可分页浏览全部文章，过滤切换清除旧游标，失败可重试。
- 范围：deploy/watchdog.sh:5、compose.prod.yml健康约定；frontend/app/pages/admin/posts/index.vue:28、useArticles.ts现有next_page_token。现有watchdog每5分钟运行、两次失败重启，保留有限重试与状态记录。
- 依赖：既有Docker健康检查、ListArticles接口，不新增依赖、不改数据库。
- 看门狗口径：唯一权威探针为 `docker inspect --format '{{.State.Status}} {{.State.Running}} {{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' blog-backend`，复用容器内部healthcheck，不再访问宿主8000。只有容器running且health=unhealthy连续两次才可重启；healthy清零；starting/restarting/stopped/missing/no-health/inspect失败只记录告警并清零，不能推断为应用故障。flock非阻塞互斥防止cron重叠，重启后至少30分钟冷却；重启失败明确记录且不循环执行。flock缺失时退出告警，不擅自安装软件。验收模拟各状态、锁冲突、Docker不可达、冷却以及重启失败。
- 验收：模拟健康/短暂失败/连续失败/恢复和容器不存在；只有连续真实失败重启；至少120篇fixture验证分页无丢失/重复、筛选/末页/失败恢复。shell静态检查、前端lint/typecheck/build及浏览器回归。
- 回退：旧前端版本；看门狗运行时回退至只告警，不能恢复已证实的误重启探针。生产变更另按准确版本授权。

### S2 文章阅读与媒体能力

- 结果：文章目录与定位、阅读时长、摘要、封面、精选入口；后台图片上传后插入正文，可设置封面/精选。
- 范围：ArticleForm.vue:28、MarkdownEditor.vue、posts/[slug].vue、ArticleCard.vue、首页；backend Article proto/biz/repo/ent及新媒体接口，复用avatar.go:27已存在的持久卷与UUID存储模式。
- 依赖：S1；现有Markdown清洗、后台权限、备份uploads卷。图片只接受经魔数验证的JPEG/PNG/WebP，沿用2MiB限制；不增加SVG/任意文件上传，不下载用户任意外部URL，不自动删除旧文件。
- 验收：目录锚点唯一/中文标题/无标题/手机布局；阅读时长边界；封面与精选刷新后持久；匿名/读者上传拒绝、伪格式/超限/穿越拒绝；真实上传落盘回读，正文图片在公众号草稿转换路径兼容。文章已有状态/发布入队不回归。
- 回退：新增可空/默认字段的扩展式迁移，旧文章可读，关闭新UI；保留数据与文件，不用down migration丢数据。迁移前备份与恢复演练在独立本地MySQL执行。

### S3 作品集与关于页面及后台维护

- 结果：作品集从后台新增/编辑/排序/上下架，支持真实截图、链接、职责与结果；关于页可编辑简介、技能、经历、联系信息；文章/作品/关于形成一致的页面布局。
- 范围：data/projects.ts:1现有3条资料、pages/projects/*、ProjectCard.vue、about.vue:4；新增admin页面、Project/About proto/biz/repo/ent、授权策略及扩展迁移。
- 依赖：S2图片能力；既有admin鉴权。现有真实文案与链接幂等迁入，新库缺记录时明确处理；不编造经历、截图或成果数字，不把模板示例当已验证事实。
- 验收：后台保存刷新持久、权限403/401、排序/下架公开隔离、链接协议校验、图片回读；原3个项目及关于内容保留且幂等导入不覆盖编辑；桌面/390px、亮暗主题、空状态/加载错误与重试。正式素材缺失时使用可解释的现有内容/占位，不假造证明。
- 回退：保留新增表和媒体，前端回退至旧静态源；数据导入仅补缺，不覆写已有后台编辑。上线前数据库备份和迁移兼容检查。

### S4 SEO基础配置

- 结果：robots.txt、sitemap.xml可访问；公开页面有对应摘要、canonical与分享标题/摘要/封面；私密页面保持noindex。
- 范围：app.vue:15、Nuxt server/routes、公开pages及S2/S3新内容接口；现有HTTPS origin配置复用，不复活RSS。
- 依赖：S2/S3公开内容模型；不新增SEO npm依赖。
- 验收：robots/sitemap200正确Content-Type，XML可解析；分页穷尽已发布文章/公开项目、XML转义和HTTPS canonical；不泄露草稿、后台、账号、收藏或查询token；公开页面SSR摘要/分享字段符合对应内容，删除/下架内容不出现在新地图；服务失败不假造空的成功地图。
- 回退：独立撤回新增路由/meta，保留原canonical和私密noindex，业务内容不改变。

### S5 邮箱验证与密码找回

- 结果：注册邮箱可验证、设置页显示验证状态和重发；登录页可找回密码，邮件链接一次有效；旧账号不因新增功能被自动锁出。
- 范围：auth.proto:17、biz/auth.go、user schema、新token repo/邮件Sender/config及授权策略；register/login/settings/验证/重置页面。复用Redis、密码散列及现有会话模型；使用可配置SMTP接入此前选定的DirectMail，不引入新依赖；实施前核实官方发送/域名要求。
- 依赖：S4公开HTTPS origin、可用发信身份与服务端私密凭据。目前只确认项目无邮件实现，不推断服务账号或域名认证已配置；开发使用fake sender，真实联调待发信条件具备并授权。
- 邮箱状态机：新注册及已有账号初始email_verified=false，不冒充已验证；保持目前登录、评论（既有审核/限流）、收藏及微信绑定可用，设置页提示待验证。验证邮件token有效24小时、找回密码token有效15分钟；每类每账号仅一个有效token，重发吊销前一个，发送失败可在冷却后再发。重发冷却60秒，验证邮件每账号每小时5次；找回请求每邮箱每小时3次、每IP每小时5次，所有账号存在性响应一致。成功验证变为verified；成功邮件重置同样证明邮箱控制权，设为verified并吊销该账号所有恢复/验证token与refresh会话。普通旧密码改密不改变验证状态。邮箱地址修改不在本轮范围。上述状态、限流阈值、过期/重发失效和发送失败分别映射到可执行测试。
- 安全验收：token仅存哈希、短时有效、原子单次消费；找回入口存在/不存在账号返回同一安全响应且限流；无token/密码/密钥日志；链接origin固定不可由Host伪造；邮件失败可恢复不误报成功；改密吊销refresh会话，明确现有access JWT残余窗口；并发/过期/重放/跨账号负向测试。新用户邮箱验证可完成，旧用户登录/评论/微信绑定不回归；真实邮箱收到邮件且能完成一次闭环后才称发信功能上线。
- 回退：关停邮件功能入口，保留用户数据/验证字段；旧登录和改密路径继续可用。若真实发信条件未具备，单独标为未完成，不阻止其余切片验收。

### S6 收藏跨设备同步

- 结果：登录读者收藏存服务端，在不同浏览器可同步；游客保留本地收藏；提供显式导入本地收藏入口。
- 范围：useCollections.ts:3、collections/index.vue:6、ArticleCollection.vue；新增Collection proto/biz/repo/ent及权限/迁移。
- 依赖：既有账号JWT与公开文章；S5账号回归通过。按身份取user_id，不允许客户端选择其他用户；只收藏公开文章，唯一约束保证幂等。
- 验收：两浏览器同账号同步，跨账号隔离，重复收藏/取消幂等，游客数据保留；导入由用户显式触发、仅成功确认后更新本地状态，部分失败可重试；退出不泄露上个账号收藏，下架文章不泄露私密内容；分页和网络失败恢复。
- 回退：关闭云端入口、游客localStorage仍可用；不删除收藏表或他人收藏数据。迁移前备份与本地恢复验证。

## 总体验收与完成标准

每片针对行为运行最小回归，再按影响运行Go build/test/vet/golangci-lint、pnpm lint/typecheck/build、proto/ent/wire再生成、contract及browser smoke。新增迁移在本地真实MySQL验证升级、旧数据兼容和从备份恢复；仅重启任务拥有的本地服务。运行git diff --check、权限与敏感信息检查。独立审查重点为账号恢复、媒体权限、公开内容过滤和跨账号收藏隔离。

代码/本地验证通过属于local_verified，不冒充已生产交付。远端Git/CI/准确SHA部署/生产迁移另走manifest与delivery_guard及逐次授权。正式验收包含新鲜线上公开/负向检查、数据兼容和邮件真实闭环；服务状态、提供者权限、备案状态不得从旧文档推断。

完成后同步TASKS、契约、迁移/恢复runbook和稳定摘要；既有文档待办存在与最新生产不符的HTTPS/公众号条目，只按本轮确认事实修正，不顺带提交其他任务记录。

## 当前限制与非目标

个人公众号没有自动公开发表权限；公安备案需作者处理，不能由代码生成备案号；小程序账号/提审不在本轮。定时发表、文章版本历史、账号删除、付费订阅、任意文件媒体库不是本轮已确认内容，不引入。仅使用已有依赖和标准库，外部邮件库确需新增时说明理由并另确认。

## 独立复核修订

2026-10-15 已落实复核提出的两项：S1固定Docker权威探针、状态分类、互斥和冷却；S5固定邮箱验证状态、旧账号兼容、重发/限流及重置后验证状态。用户已确认六片；独立复核OKAY，按S1起开始实现。

## S1本地验收（2026-10-15）

看门狗改为Docker健康状态、互斥、失败分类、30分钟冷却和只告警回退。10项本地无Docker副作用回归通过；先复现healthy误重启，以及独立审查发现的损坏时间1:2/08误重启，再修复至green。bash -n、diff-check通过，独立审查APPROVE；shellcheck未安装，以语法检查、行为回归与独立审查替代。

文章后台120篇browser fixture通过：50/50/20三页无重复、末页禁用、503恢复、过滤清游标、旧响应隔离、发布从草稿筛选移除（末页10→9）、删除后边界补位（9→8）。pnpm lint/typecheck/build、node --check全通过，独立审查APPROVE；fixture服务已停止。新代码尚未远端交付或生产部署。下一片S2文章阅读与媒体。

## S2本地验收（2026-10-15）

新增Article可选cover_url/featured、ADMIN图片上传/公开回读及v12扩展迁移；旧mask保留与明确空值/false清除、published边界、精选过滤和真实多条排序/分页均有回归。存储失败typed错误不暴露内部path。Go全量test/build/vet、golangci-lint(0 issues)、关键race、生成稳定、独立backend/frontend审查通过。buf lint沿用既有全仓AIP命名违例，未将其冒充通过；未改变该既有门禁。

真实MySQL8.4通过11→12、旧行保留、默认空封面/非精选、metadata写入、mysqldump+gzip恢复与恢复后再次升级。前端lint/typecheck/build、presentation utility、article_experience mock browser、S1分页回归通过。真实HTTP+SQLite/miniredis/tempFS/fake微信及BFF browser闭环通过：登录/刷新→PNG转JPEG上传→实际文件回读→正文插入→封面/精选保存与刷新→匿名详情图片解码/目录/阅读时长；匿名上传401、Reader403且无写入。fixture已停止；preview已停止。所有新功能仍为local_verified，未远端交付/生产更新。
