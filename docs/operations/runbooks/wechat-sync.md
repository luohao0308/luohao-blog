# 公众号同步启用与恢复

适用范围：博客后台发布到微信公众号，支持草稿、公开发表、群发三种模式。和小程序登录凭据独立。仓库默认关闭，测试使用本地模拟微信接口；生产已启用 draft 模式，具体交付/域名切换状态以 TASKS.md 为准。

## 前置条件与配置

- 确认账号主体、认证和相应草稿/发布/群发接口权限，依据当前开发者平台而非历史截图。
- 配置公众号 AppID/AppSecret、博客公开 origin、微信永久封面素材 ID。
- `default_thumb_media_id` 必须是微信永久素材的 ID，不是图片 URL、临时素材或本机路径；所有文章使用这个默认封面。可以使用下面的准备命令自动生成并保存 ID，不必在后台手找。
- 服务器公网 IP 如受微信接口白名单限制，先在开发者平台核实白名单。
- 正文图片支持 JPG/PNG，限 1MiB；本站公开图片域名默认允许，其他域名需加入 `image_hosts`。拒绝内网、回环、链路本地地址和跳到不允许域名的重定向。
- 博客标题允许比微信更长，公众号图文标题最多 32 字；超长同步会明确失败，博客发布仍可成功。

只在服务器的 Git 忽略密钥文件中配置，不通过聊天提交凭据。模板：`backend/configs/secrets.local.example.yaml`、`deploy/prod-conf/secrets.local.example.yaml`。配置块：

```yaml
official_account:
  enabled: false
  app_id: "<公众号AppID>"
  app_secret: "<在服务器填入>"
  site_url: "https://<博客域名>"
  default_thumb_media_id: "<微信永久封面素材ID>"
  mode: draft
  image_hosts: []
```

`mode`：`draft` 仅保存草稿，`publish` 公开发表但不群发，`mass` 向全部关注者推送。选择 `mass` 会消耗群发额度，不能把发布和群发串成同一次任务；草稿可能在发表/群发后被微信移除。公众号通常有成功群发频次限制，服务号还受每月次数限制；以当前平台额度为准。API 群发保护若开启，仍可能需要公众号管理员确认。

## 自动准备默认封面与公开地址

源码自带默认封面 `backend/internal/data/assets/official-cover.jpg`（900×500 JPEG、约13KB）及SVG源。封面通过微信永久素材接口 `type=thumb` 上传，满足 JPG/≤64KB；不调用草稿、发表或群发接口。

先在 Git 忽略的 secrets.local.yaml 中保存 `official_account.app_id/app_secret`。在 `backend/` 执行（site-url 应先核实当前公开入口）：

```bash
go run ./cmd/wechat-cover -conf ./configs -site-url https://makerhao.cn
```

程序只输出成功状态、非密钥素材 ID 和公开地址，不输出 AppSecret、Token 或带鉴权参数的 URL。`-check-config` 仅输出两个字段是否存在，用于诊断是否保存到了正确配置段。

成功后保存两份本机产物：

- `data/uploads/wechat-cover-setup.json`：账户/图片指纹与上传结果；同一图片/账号重跑不会重复上传。已开始但结果不确定会阻止重跑，先在公众号后台核对，不要盲删 journal/lock。
- `configs/zz-official-account-setup.local.yaml`：只有公开AppID、site_url、封面media_id，按文件顺序覆盖空配置，不修改用户的密钥文件，也不启用自动推送。已通过真实配置加载器的合并回归测试。

部署时可使用 `-conf ../deploy/prod-conf` 并将 state 文件放在持久卷中；两个配置目录的生成片段均已被 Git 忽略。本机生成片段不等于生产部署，需沿用正式交付流程。

明确失败码40164表示请求出口IP不在微信白名单；40243表示AppSecret冻结。按平台要求修复后可重试；上传超时/不确定结果先人工核对，禁止重传造成重复素材。没有真实成功回包时不能声称素材ID已准备完成。

## 真实草稿联调（不公开发表）

`wechat-draft-check` 只读取配置origin上的已发布文章并创建公众号草稿，不启动服务/任务worker，不调用发布/群发接口。显式指定一篇可公开联调的文章：

```bash
# backend目录；所有真实凭据仅在ignored配置文件中。
go run ./cmd/wechat-draft-check -conf ./configs -slug <已发布文章slug>
```

默认正文采用博客缓存HTML，默认永久封面、作者和原文URL取当前配置。最多60秒；只输出草稿ID/标题/slug/原文URL及安全错误码。`data/uploads/wechat-draft-check/` 的账号+版本journal持久记录结果，同版本成功复用，未知结果禁止重试；不是公众号发表状态。

成功后在该公众号草稿箱审阅排版、封面和原文链接；不自动删除测试草稿。确认内容后再按已批准的发布/群发策略启用worker。平台不允许该接口时保持关闭并记录安全错误码，不通过后台浏览器绕过权限。

## 启用与验证

1. 交付时先审阅 migration `000011_wechat_sync_tasks`，执行既有备份流程。迁移新增任务表，不改文章数据；应用启动时自动运行版本化迁移。
2. 按已授权的发布流程部署准确版本；本 Runbook 本身不授权部署或修改公众号设置。
3. 先配置 `mode: draft`，确认全部必要字段后设 `enabled: true`，按发布流程重启后端。启用却缺少凭据/公开地址/封面，或模式不合法时启动明确失败，应先修配置而不是绕过校验。
4. 后台 `/admin/wechat-sync` 核实状态。用一篇可公开的短测试文章创建→发布，验证文章“阅读原文”链接及正文图片、标题和默认封面。已有文章不自动全量推送；可用后台 slug 输入框显式加入。
5. 验证完成后按用户确定的方式切换 `publish` 或 `mass`。这会开启真实发表/群发，须具备针对该公众号的外部操作授权。
6. “等待微信结果”只表示提交成功。仅轮询确认终态后显示成功，任务保留草稿 ID/远端 ID；最终信息可在微信后台核对。

## 行为与恢复边界

- 非 published → published 时，文章写入与任务入队处于同一数据库事务。微信网络请求在独立 worker 中进行，远端失败不回滚博客发布；数据库事务失败会回滚文章和任务，不能假报已发布。
- 已发布文章编辑不自动重复通知关注者。需要同步新版本时由后台显式加入；同一账号、配置和内容版本幂等，阅读计数变化不生成新任务。
- 后台管理接口只允许管理员。接口返回运行状态与安全错误码，不返回 AppSecret、access token 或完整网络错误。
- 明确失败且未获得远端提交 ID的任务可重试；保留草稿 ID，重试提交不会重新创建草稿。源文章变化时应同步新版本，旧任务取消。
- 状态查询网络/Token/限额等临时失败持久化下次查询时间，按 1–30 分钟退避；Token明确失效时清缓存，下次重新获取。权限/远端对象等永久错误转结果未知并保留具体安全错误码，停止自动查询；连续错误达到100次领取后停止自动查询。只读查询期间进程中断恢复为 polling，保留远端 ID；不重新提交发布。
- 草稿创建或发布提交超时/断连/畸形响应，及写操作中进程退出：结果未知。不得直接重试；先在微信后台核对草稿/文章/群发记录，必要时由管理员手动完成。首版没有未验证的“强制重发”按钮。
- 更换公众号 AppID 后，旧任务不会调用新账号：无远端副作用的旧任务取消，有草稿/提交 ID的旧任务转结果未知，在原公众号核对。账号指纹存储 AppID SHA256，不存凭据。
- 不自动删除公众号草稿或撤回已发文章。博客下架也不会自动撤回公众号内容；撤回是另行授权的操作。
- 关闭 `enabled` 后暂停处理任务并保留状态。恢复同一配置后继续已知的未完成任务。

## 回退与证据

关闭同步是最小回退：`enabled: false`，按既有流程重启后端；保留文章与任务表。不要运行 down migration 丢弃历史任务，除非明确获批且已有备份；已发表/群发的内容不因关闭同步自动撤回。

可复现验证：`go test ./...`、`go test -race ./internal/biz ./internal/data`、`golangci-lint run`；前端 `pnpm lint && pnpm typecheck && pnpm build`。浏览器 fixture 见 `scripts/e2e/README.md`；它只调用本地微信模拟服务，不消耗真实额度。

官方接口：[新增草稿](https://developers.weixin.qq.com/doc/service/api/draftbox/draftmanage/api_draft_add)、[提交发布](https://developers.weixin.qq.com/doc/service/api/public/api_freepublish_submit)、[查询发布](https://developers.weixin.qq.com/doc/service/api/public/api_freepublish_get)、[群发](https://developers.weixin.qq.com/doc/service/api/notify/message/api_sendall)、[群发结果](https://developers.weixin.qq.com/doc/service/api/notify/message/api_massmsgget)。

## 备案后公开 origin 切换

主站 `https://makerhao.cn` 的 TLS/公开资源验收通过后，使用 `deploy/official-account-domain.example.yaml` 的单字段片段，保存为生产 `deploy/prod-conf/zzzz-domain-origin.local.yaml`（Git 忽略，0600）。模板放在配置目录外，避免 Kratos 自动加载 example。真实 loader 以文件名排序合并；确认没有更晚的覆盖文件重写 site_url。

该片段只设置 `official_account.site_url: https://makerhao.cn`。先备份完整配置，再针对已授权准确版本更新并重启 backend；用同样的 Kratos file+env loader只投影 scheme/host、enabled/draft 状态，不输出 AppSecret/token/封面等配置内容。验证新任务 original URL 为 `https://makerhao.cn/posts/<slug>`、本站图片允许来源与原有外部 image_hosts。

不通过 cover 准备命令变更域名，不重传封面或历史草稿；SiteURL参与任务幂等ID，切换时不批量重建旧任务。现有草稿旧IP原文地址由主站跳转兼容，新发布事件生成HTTPS原文地址。

切换前必须按状态只读统计同步任务。`SiteURL` 包含在任务 ID：旧 `queued/ready` 在处理时会因当前 ID 不同而转为 `cancelled`（ready 的已有草稿 ID 保留，不撤回远端草稿）；旧 `failed` 会因 ID 不一致不能重试；`polling` 正在确认远端结果，不能盲重建或取消。

生产切换门：`queued/ready/drafting/submitting/polling/checking/failed/unknown` 合计必须为0；否则停止配置变更，在当前origin下先完成或逐项审查现有任务。切换窗口内先停止唯一 backend，再复查上述计数仍为0，随后应用配置并重启，避免检查与切换之间写入旧任务；若复查非0，恢复原backend并停止切换。这个统计只是读取，不能为放行修改状态、删journal、批量重试或同步历史文章。每次新建发布事件用HTTPS origin生成新ID；已经drafted/published的历史记录及远端草稿保持原样。
