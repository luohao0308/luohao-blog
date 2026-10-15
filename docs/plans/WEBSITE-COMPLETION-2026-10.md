# 网站能力补齐（2026-10-15）

status: production_deployed
delivery: production_deployed
confirmed: 2026-10-15 用户确认全部六片、按顺序实现
current_slice: none
S1-S6: production_deployed
S5_mail_delivery: inbox_confirmed
S5_production_account_email_journey: production_verified
mode: direct execution; slices approved by user
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

## S3本地验收（2026-10-15）

Project/About机器契约、后台新增/编辑/排序/软下架、图片与公开隔离已实现，原三项目和About以幂等seed迁入。字段/嵌套JSON边界、URL协议、不可变slug、mask替换/清空以及匿名401/Reader403均有回归。Go全量test/vet/build、lint0、四包race和生成稳定通过；根复跑Project/About四包回归通过，独立backend/frontend审查APPROVE。真实MySQL11→13及12→13验证旧文章非默认封面/精选保留，seed重跑不覆盖编辑，SQL备份恢复和重升级通过。

真实HTTP+BFF浏览器PASS：登录→创建草稿→匿名404→上传PNG并实际落盘回读→上架/排序→移动详情→软下架公开404→About编辑并刷新持久。联调暴露SSR hydration忽略payload与SPA AsyncData缓存两项真实问题；修复后同一page验证404草稿上架后SPA可见、200项目下架后列表/history不残留、已加载旧About编辑后SPA显示新内容。公开请求no-store，hydration复用payload，后续导航重取；单一NForm同时锁定按钮/Enter提交。最新前端专项mock及S1/S2回归、lint/typecheck/build全部通过；浏览器PASS不能被fixture正常退出替代。新增复跑说明见scripts/e2e/README.md。所有变更仍仅本地，未远端交付或生产部署。

## S4本地验收（2026-10-15）

新增robots.txt、sitemap.xml、无依赖SEO helper与公开页面SSR title/description/canonical/OG/Twitter信息。图片为受控上传路径或原有品牌封面静态副本；origin固定runtimeconfig，不从Host/转发头派生。私密后台/账号/收藏以及未来恢复页面noindex并排除地图。robots只约束抓取，不将其当权限保护；不挡本站公开分享图片。

测试先复现S3构建robots404，以及siteURL双斜杠路径可逃逸到外域，再修复为green。fresh pnpm lint/typecheck/build、纯utils边界、独立SEO专项通过：121文章/111作品多页、draft/hidden防御过滤、无caller Cookie/Authorization转发、XML实际解析/转义/真实lastmod、下架后fresh重生成、后台故障/重复token/空页续页/15s挂起上游在10s超时后503，错误和正常地图均no-store。首屏HTML与浏览器DOM对照确认摘要/分享/canonical/private noindex。collect原始记录与页数限制在拉取阶段生效，最终XML限制50000URL/50MiB；没有开启RSS。独立review APPROVE，所有专项自有进程均已退出。仍为local_verified，未更新生产。

## S5实施细化（原批准范围内）

复用现有Argon2id HashPassword/VerifyPassword，不改变散列算法；新register/update/reset密码最大1024 UTF-8 bytes，服务端最小值沿用8bytes，旧Authenticate/current password不新设max以保留历史长密码。前端既有8位最小检查与服务端Unicode口径存在旧差异，不伪称完全一致。

v14新增users.email_verified(defaultfalse)、credential_version(default0)，并增加account_email_tokens（user/purpose唯一、SHA256 token_hash唯一、issue时version、expiry）。采用同一SQL事务条件消费token与用户CAS/验证/改密及清理，两者失败同时回滚，避免Redis令牌先消费而数据库改密失败的跨存储窗口；Redis继续承担fail-closed冷却/小时额度。只存hash，raw只在内存和邮件；重发替换旧hash，失效/并发/重放由目的和版本条件保护。普通改密不标记verified，但版本增加并撤销旧refresh/token；邮件reset标记verified并增加版本。

找回请求使用有界内存队列和固定worker，HTTP不等待已知账号SMTP，已知/未知同样200 Empty受理响应，避免投递时延泄露是否注册；未配/队列满全体同503。队列不持久化raw token，worker才查用户并生成链接。SMTP失败仅精确回收本次hash，不能删掉之后的新token；不记录收件人/密码/token/provider原始敏感错误。SMTP总deadline覆盖连接后的协议阶段，worker可关闭。接口只表述受理、不声称投递成功；真实邮箱闭环仍需已验证发信身份和授权。

## S5本地验收（2026-10-15）

v14账户字段和目的隔离hash-only token表、验证/重置四端点、versioned refresh、bounded reset worker、严格TLS SMTP与前端三私密页已实现。普通改密与后台入口统一到个人设置，当前账号保存后登出；公众token确认不归因到另一个已登录账号。旧登录/评论/微信不受未验证状态限制；旧refresh UUID值兼容v0，旧超长密码仍可登录；access JWT残余窗口为配置TTL（默认15m），UI和契约如实提示。

先锁定失效/并发/限流/存储和发送失败，再实现。独立复核修正：SMTP总deadline/ctx取消、异步发送避免时延枚举、非法token先便宜判定、容量1重置散列防并发内存放大、MySQL no-op affectedRows语义、真实私密enabled覆盖、移除无需求Ent upsert生成膨胀。fresh Go全量test/vet/build、lint0、四包race、api/config/ent/wire重复生成逐字稳定、无依赖变更、backend/frontend独立review APPROVE。前端fresh lint/typecheck/build/mock通过；后台旧modal缺少统一退出入口先RED，删除重复表单复用/settings#password后GREEN。

真实MySQL13→14和11→14：旧账户密码散列/头像/微信绑定保留，verified=false/version0；actual TokenRepo确实RUN/PASS非skip，同秒Issue/已verified确认、重发/CAS/重置/普通未验证改密语义通过；user-purpose/hash唯一、FK拒孤儿，升级前备份恢复重升级与升级后verified/version/hash记录备份恢复均通过。真实HTTP+BFF+SQLite/miniredis/fakeMailer browser PASS：动态注册A/B/C，A验证/重置、B身份确认A链接后B仍false、fragment即时清理且不进DOM/localStorage/network URL、目的混用/弱密码/重放400，A旧refresh401/Brefresh仍200、旧密码401/新密码200、C未验证邮箱reset变true、known/unknown相同受理响应、三页面SSR noindex。所有fixture/preview已正常停止。

SMTP真实身份/配置状态仍未提供，实际投递闭环标记pending_external；默认disabled并不锁旧服务，不能将fakeMailer验证冒称真实发信上线。按原计划继续S6，不阻断其余本地验收。

## S6本地验收（2026-10-15）

新增三authenticated收藏端点、v15表及claim-only tenant边界，ADMIN/Reader均published-only；唯一约束确保并发幂等/首次日期，SQL过滤先于分页，稳定排序且private响应no-store。Web登录云收藏、游客原blog:collections ISO时间结构、显式导入/成功项确认后持久清理、失败保留/退出清cloud但保留guest；小程序本地收藏未扩展。

fresh Go全量test/vet/build/lint0、四包race、api/ent/wire重复gen稳定、backend独立review APPROVE；实际MySQL14→15/11→15验证两个仓储test确RUN非Skip、8并发幂等/同秒排序/hidden预过滤/tenant、user/articleFK和unique、旧verified/version/media保留及升级前后SQL备份恢复。frontendfresh lint/typecheck/build及独立review APPROVE。

先红后绿锁定：等待初始session/cloud成功才能ready，详情503 alert/disabled/retry；loadVersion+mutationEpoch避旧same-owner快照回滚；superseded force initialize不永久禁有效状态；localStorage quota保留guest；watchflush同步identity和状态；authFetch expectedUserId剥离且旧401不能重试/清新B；login/register等已有refresh，actual deferred mock证明B HTTP晚于A refresh、最终cookie和可见菜单B、A不写B；busy只释放当次Set。

Root真实BFF/SQL完整collections_live_e2e PASS：未验证账号可用，105收藏50/50/5、重复saved_at不变、client userID不窜号、不同账号删除隔离、同账号两浏览器、explicit部分导入保留hidden项、SPA感知另一浏览器取消/重新收藏、下架private title/content不返回、hidden/absent取消幂等、完整退出仅guest可见、匿名本地收藏、390px无溢出、noindex。初期用例错误(一draft一deleted、同秒slug排序、退出menuitem语义)已纠正，只有最终完整script PASS才计验收。

最终同构建回归：S1后台120分页、S2文章体验、S3作品/About、S5邮箱、S6收藏mock全部PASS，S4独立Nitro SEO再验PASS；S2 actual media上传/FS/metadata/public/auth拒绝又PASS。邮箱mock原本首个/login就模拟已登录导致合法重定向与点击竞态，已修成先anonymous验证forgot入口再显式authenticated设置流程，保留全部断言而非跳过。所有fixture/preview已正常停止，测试容器仅任务专属MySQL保留数据待交付准备后停止。

六片本地验收完成；真实DirectMail身份/投递仍pending_external，新版本远端Git/CI/生产部署/迁移尚未授权和执行。备份与保留数据回退说明见docs/operations/runbooks/website-completion.md。


## 生产交付结果（2026-10-16）

六片实现经 [PR #95](https://github.com/luohao0308/luohao-blog/pull/95) 合并；发布门禁发现并发测试启动通知可能丢失，经 [PR #96](https://github.com/luohao0308/luohao-blog/pull/96) 修复，500 次重复、50 次 race、完整后端检查及独立复核通过。两次交付及最终生产更新分别取得准确版本授权，没有改变持久权限策略。

生产版本为 `30e291dc2167229e6018bd3cd3e1a540a7607ea0`，源码树等于批准源 `28bbf87055efc14f2da3dfe4587aae8d1bda76e5`；[release](https://github.com/luohao0308/luohao-blog/actions/runs/38772498032) 与 main CI 通过。前后端镜像按原始 OCI index digest 验证并固定版本；镜像仓库直连缓慢时，仅替换传输方式，所有索引、证明、配置及层内容仍逐项 SHA-256 核验。应用升级只执行一次，基础容器未重建。

生产 SQL、uploads 与私密配置备份已校验，未轮转删除历史备份；数据库 v11→v15 且 dirty=false，原账号、文章及头像保留。私密覆盖、公众号草稿配置、HTTPS origin 与 Secure Cookie 保持。新看门狗已回到正式 cron 路径，健康后端不再因宿主端口误探测而重启。

生产 42 项公开页面/API/SEO/访问控制检查与 8 个桌面1440px/手机390px页面通过，另有可信 TLS/备案/cookie 冒烟和截图人工检查。DirectMail 已启用，唯一配置测试邮件由用户确认进入收件箱。生产真实账号验证/找回邮件链路随后另获授权并完成，见下节；管理员写入流程未在本次额外执行，不将本地真实 SQL/BFF 流程描述为生产管理员写入验收。仍不执行 down migration、数据库覆盖恢复、额外发信或公众号公开发表。

一次性流水、备份位置、digest、凭据操作审计及截图保留在受控服务器和本机 ignored `.dev-workflow/`；可重复部署与保留数据回退规则仍见 [Runbook](../operations/runbooks/website-completion.md)。以上交付结果优先于先前各切片当时的本地状态记录。


## 真实账号邮件验收（2026-10-16）

用户另行明确授权为自有邮箱创建并保留一个 READER 验收账号，验证和找回邮件各一封。账号通过公开注册 API 建立，初始随机密码、JWT 和刷新 Cookie 仅保留在一次验收进程内存中；用户在真实邮件中点击确认验证，并自行在网站设置新密码、完成登录，密码及邮件 token 未提供给 Agent。

第一封找回链接按设计在15分钟后到期，服务端仍为凭据版本0，没有发生密码重置。经用户额外授权补发一封后，真实验证与找回闭环完成：邮箱已验证、凭据版本升至1、该账号邮件 token 计数为0；原密码登录和原刷新会话均返回401，用户确认新密码登录，服务端查到新会话。验收进程随后正常退出，未持久化密码或 token，未修改其他账号、删除数据或追加后台写入。本轮账号验证邮件请求1次、找回请求2次；这与部署阶段已确认收件的配置测试邮件分别计数。

保留现有 access JWT 的短 TTL 设计，不宣称其全部即时撤销；本次实际验证的是原刷新会话失效。账号按授权保留为普通读者，既有生产版本 `30e291dc2167229e6018bd3cd3e1a540a7607ea0` 未改变。具体单次授权、时间、服务端状态与安全输出存于本机 ignored 验收记录，不在源码保存收件人、凭据或邮件链接。
