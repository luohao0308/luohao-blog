# 正式小程序接入与身份隔离

status: approved
parent: [后台与小程序收尾](ADMIN-MINIPROGRAM-CLOSEOUT-2026-10.md)，S2 → S3
formal_app_id: wxf27116cc66cc49a4
legacy_preview_app_id: wx58089476f518fd3f
current_slice: F2 (formal preview and human binding acceptance pending)

## 目标与已确认事实

用户已选择正式发布到「浩的碎片集」wxf27116cc66cc49a4。旧wx580预览的打开、阅读、标签、搜索、收藏/取消、AI回答与来源跳转、微信登录账号及头像均由用户确认正常；该结果不替代正式AppID的身份/域名与设备验收。用户的正常显示结果优先保留，昵称头像wire映射修复属于代码/fixture发现，不标为手机已复现故障。

当前生产后端源码90ca4f8、前端源码b64a55c，标签筛选修复已验收。正式平台本轮只读确认：个人主体、工具/信息查询、微信认证完成、备案管局审核中、隐私指引已更新但内容未逐项核对、UGC未声明、服务器域名尚未配置。没有更改平台设置或读取AppSecret。

当前users.wechat_openid不区分AppID，绑定会覆盖账号原openid；Redis绑定票据只有openid，没有AppID。正式切换会改变身份来源，需先隔离身份及票据，不能仅替换AppID/Secret。保留网站账号ID、密码、角色、头像、文章和旧绑定，不按旧openid或昵称自动合并身份。

## 三个有序切片

| 切片 | 结果与修改范围 | 依赖 | 可执行验收 | 回退/停止点 | 状态 |
|---|---|---|---|---|---|
| F1 身份隔离与本地验证 | 后端新增v16身份表、AppID范围查询/绑定/票据及受控旧身份回填工具；保留旧列；合入已有昵称头像边界修复；项目AppID对齐正式号。无生产变更 | 本计划确认；确认旧身份来源，不能猜测其他环境 | MySQL v15→v16/回填幂等和冲突；Redis真实TTL/GETDEL；跨AppID/禁用/并发绑定和HTTP回归；小程序typecheck/test；独立审查 | 本地提交可回退；不使用真实用户凭据/数据进行单元测试 | completed |
| F2 正式配置与真机 | 准备并在准确版本授权后配置正式号request域名及必要声明，备份SQL/uploads/私密配置；暂停微信绑定写入窗口，迁移并显式回填旧身份，切正式凭据/后端与同源码正式预览 | F1通过；正式Secret由用户存私密文件；准确源码/配置/迁移/平台/预览操作授权 | 确认生效AppID；正式微信首登经用户在页面输入原账号密码新增身份，再次登录；用户UUID/密码/内容/旧绑定保留；普通真机联网、昵称头像及核心流程；不复用旧预览证据 | **保留支持v16的新后端及数据库**，关闭微信登录/绑定或恢复已核验旧AppID配置；不下迁移、不恢复覆盖数据库，不直接回退v15镜像 | in_progress |
| F3 开发版本、提审与发布 | 绑定正式AppID、准确源码树、版本及包证据，分别上传、提审、发布；不将预览或上传视为已上线 | F2通过；平台当时允许相应操作；对应准确包和操作授权 | 备案/域名/隐私内容/类目及适用声明核对，审核结果和正式版本记录，发布后真实入口冒烟 | 不通过则保留现有状态；正式发布前必须有有效备案结果；平台版本回退另按实际可用能力及准确授权办理 | pending |

同一时刻仅一个切片in_progress。计划确认不替代远端Git、凭据、生产迁移/回填、平台写入和发布的准确版本操作授权。确认前仅进行只读核查及计划文档准备，不改产品配置或生产数据。

## F1 最小身份设计

新增wechat_identities表，字段沿用项目UUIDv7、user_id外键、大小写敏感app_id(varchar32)/openid(varchar64)、created_at。唯一约束(app_id,openid)和(user_id,app_id)。运行时仍只配置一个小程序，不新增多小程序管理界面。

旧users.wechat_openid原样保留。v16 SQL只建表，不硬编码环境AppID；受控回填工具要求明确legacy_app_id，先dry-run、核验生产旧身份实际来源，再在暂停绑定写入窗口事务回填。相同用户/AppID/openid幂等跳过，任何归属冲突停止，不更新旧列、不覆盖新表中的已有归属。完成后核对旧列不变、旧身份映射覆盖关系；未知来源不得自动套用wx580。

新登录仅查询服务端当前AppID的身份；不回退裸openid。业务身份携带AppID，绑定票据使用v2且按AppID隔离的Redis命名空间，保留一次性GETDEL和10分钟TTL，拒绝旧格式、跨AppID、重放和畸形票据。Login/Bind都先检查配置完整，禁用微信登录同时禁用绑定。

绑定继续使用现有邮箱密码页面，只插入新关系：同一身份与账号幂等；身份属于别的账号、同账号同AppID已有不同身份均409，不做替换式upsert。事务/唯一约束兜底并发；绑定时在同一MySQL事务内锁定目标用户、复核密码验证对应的credential_version并插入身份，密码重置后的旧验证结果不能建立新身份。Login/Bind签发会话后，复用现有站点登录的凭据版本复核；发现版本变化则删除刚签发的refresh会话并拒绝返回，不只在身份插入前做CAS。原账号在旧AppID的关系保留，正式号通过用户重新认证新增行。

## 测试与生产证据

- 跨AppID的相同openid字符串不自动登录或绑定为同一账号；AppID只由服务端配置确定。
- 新增正式身份后网站UUID、密码、角色、内容、头像及旧绑定不变。
- 两账号争同一身份、同账号争两个身份最多一个成功；精确重复幂等，失败不覆盖成功关系。
- 错误密码、旧票据、跨AppID票据、重放/过期、禁用/缺配置均拒绝；密码重置与绑定竞态不允许旧凭据新增身份。
- 真实MySQL迁移和回填测试包含已存在身份、冲突停止、旧列保留及重复执行；另外使用真实MySQL事务验证两账号争同身份、同账号争不同身份，以及密码版本更新与身份插入的两种提交顺序，证明锁/CAS/插入原子性。要求实际RUN/PASS，驱动不可用或静默Skip不得放行。复用既有隔离数据库迁移演练工具，不使用生产快照做本地fixture。
- 演练v16数据库上新后端关闭微信时网站邮箱登录/读取仍正常；恢复旧AppID配置后，已回填旧身份可查询且正式新身份行仍在；签发后版本变化清理新refresh的负向测试需要实际Redis路径。
- 正式号首次绑定和再次登录由用户在页面正常完成，密码/验证码/完整认证URL不进聊天或日志。真实点赞/评论验收要另明确对象、内容和次数，不沿用已结束的旧生产验收写入额度。

## F2 停写与启用的固定顺序

采用新后端明确的微信enabled开关（Login和Bind共用守卫），不靠清空旧Secret暂停。旧后端没有该守卫，因此：

1. 核验旧运行AppID来源、准确镜像/源码和备份，记录唯一旧backend容器及服务实例清单；先以本地/演练环境验证准备启用的v16镜像。
2. 正常停止旧backend并给在途请求排空时间，核实旧容器已停止、没有其他旧版本微信写入实例；不会用“新开关关闭”代替旧实例停止。该切换有短暂后端不可用窗口，其他容器不更改。
3. 使用已准备配置启动v16后端，微信enabled=false且旧AppID元数据保留；自动迁移仅在备份验证后执行。确认新进程存活、schema v16且dirty=false，微信Login和Bind均拒绝，站点普通登录/读取可用。
4. 微信关闭期间执行明确legacy AppID的dry-run及事务回填，核对旧列未变、映射完整且无冲突。失败停在关闭状态，不开放正式微信入口。
5. 仅将对应正式AppID/Secret成对设好并启用微信，重启同一v16后端，再核验实际配置AppID和正式预览。原账号绑定由用户在页面重新认证；不自动关联旧openid。

精确执行授权必须包括旧实例停止、上述两次启动/重启、迁移/回填和必要的一次补偿关闭/旧配置恢复；不从旧版的“只更新一次后端”授权推导。无在途旧写入者、关闭守卫和回填核对是开启正式入口的硬门禁。

## F2/F3 配置与数据边界

小程序与后端需成对使用正式AppID及其对应Secret；公众号official_account凭据、邮件、网站账号等其他配置保持原值。Secret仅在不提交的私密文件中处理，代理不读取或输出密钥值；提交范围只有不含密钥的示例/配置验证逻辑。运行配置更改后需重启对应后端，不能依赖热更新。

正式request域名计划为https://makerhao.cn（不带/api/v1）；其他网络域名仅按实际API用途配置。保持urlCheck=true，普通真机实测作为通过证据；没有web-view，不增加无关业务域名。隐私指引必须与实际邮箱/账号、OpenID、头像、评论及问答处理一致，UGC按真实文本和审核场景填写；不据“个人/信息查询”武断断言不能发布，也不谎报没有用户内容。

备案管局审核中不算备案成功，可继续开发联调/预览；能否提审以正式账号当时门禁为准，正式发布前核实备案完成。跨AppID不承诺自动迁移wx本地收藏，本轮不新增云收藏或批量搬移功能。

## 风险与恢复验证

1. 身份来源误判：显式legacy AppID+dry-run覆盖核对，冲突停止；不复制或推算正式openid。
2. 旧票据或并发重绑：新命名空间/配置守卫/CAS及数据库约束；执行对应负向与竞态测试。
3. v16生效后旧镜像启动失败：新后端会执行嵌入迁移，v15镜像不能识别v16。新后端已健康时，采用已有项目fail-forward口径，保留新后端和数据，仅关闭微信入口或恢复旧凭据。若迁移失败/dirty或新后端不能启动，则暂停受影响服务并记录实际状态，停止自动操作，进入另行准确授权的向前修复；不宣称此时“关闭微信即可恢复网站”，不自动启动v15镜像，绝不用force/down或数据库覆盖掩盖版本问题。参考[网站补齐Runbook](../operations/runbooks/website-completion.md)。

## 决策取舍与依据

备份后覆盖单一旧绑定的方案代码更少，但会丢失在线旧关系；只加users.wechat_app_id仍只能保留一条关系。追加身份表多一次迁移，却能在不改网站账号与旧列的前提下隔离来源并可验证恢复，因此作为推荐方案。没有选择新增多AppID前台管理或自动身份合并。

设计依据包括当前源码的User/WechatUsecase/Redis票据实现、架构只读审查，以及微信官方[登录身份](https://developers.weixin.qq.com/minigame/dev/wxcloudrun/src/quickstart/plan/login.html)、[code2Session](https://developers.weixin.qq.com/miniprogram/dev/server/API/user-login/api_code2session.html)、[网络配置](https://developers.weixin.qq.com/miniprogram/dev/framework/ability/network.html)、[隐私说明](https://developers.weixin.qq.com/doc/oplatform/Third-party_Platforms/2.0/product/privacy_setting.html)、[提审契约](https://developers.weixin.qq.com/doc/oplatform/openApi/miniprogram-management/code-management/api_submitaudit.html)、[备案流程](https://developers.weixin.qq.com/miniprogram/product/record/guidelines.html)。官方UGC强制参数的特定类目条件不扩大解释为所有个人号的禁令。


## 只读方案审查

已完成独立架构建议与对抗审查。按审查补齐F2停写/排空顺序、启动或迁移失败时的停止与向前修复边界、真实MySQL并发/CAS及签发后Redis会话清理门禁后，复审通过。审查只证明方案完整性，不替代用户计划确认或具体外部操作授权。

## 用户确认

2026-10-16 用户确认全部三片，先执行F1本地实现与验证。正式小程序AppSecret尚未生成或保存，不阻塞F1；未授权本次执行生产或平台写入。

## F1 本地验收结果

F1本地验收时尚未远端交付；现已通过下述PR交付，仍未生产部署。正式项目AppID已对齐wxf27116cc66cc49a4且urlCheck保持true；没有生成新的预览或上传包。正式Secret尚未生成/保存，F2等待凭据及准确版本操作授权。

- 关键失败测试先复现了禁用微信时Bind仍可绑定，修复后通过。
- 后端 `go test ./...`、`go build ./...`、`go vet ./...`、`golangci-lint run ./...` 通过，lint 0 issues；小程序typecheck/test通过。变更未涉及前端Web运行代码，未重复其构建。
- `python3 scripts/test_wechat_identity.py` 在脚本自行创建并清理的Docker MySQL/Redis上实际通过，无静默Skip：v15→v16 clean、原账号和文章保留、无自动回填、CLI dry-run/apply/幂等、升级备份恢复、唯一竞争、两种凭据锁顺序、实际提交后丢失确认、HTTP绑定与普通站点登录/读取、真实Redis签发后版本变化清理。
- 环境隔离回归通过；同时注入带/不带下划线的合成KRATOS数据库覆盖变量，演练仍只访问自建fixture。真实驱动检查已接入后端CI，远端CI尚未运行。
- 独立安全审查初次发现的演练环境覆盖、提交结果未知误报、Ent/MySQL外键差异均修复并复审APPROVE；配置默认false及环境显式启停也通过实际解码测试。
- 针对性MySQL/Redis/HTTP测试的race检查通过；差异检查、本地文档链接及Git边界检查通过。仅清理本轮测试容器，共享Docker及原工作树改动保留。

契约见[小程序身份](../contracts/WECHAT-IDENTITY.md)，F2执行约束见[正式切换Runbook](../operations/runbooks/formal-miniprogram.md)。生产迁移后不回退v15后端、不down/force、不覆盖恢复数据库；COMMIT结果未知须先核验，不盲目重试。

## F1 远端交付

用户针对源e45a5a70f1820bc8fc8444d51b96d339ecd1a527分别授权push、PR、merge及合并自动镜像发布，未包含生产或微信平台写入。[PR #99](https://github.com/luohao0308/luohao-blog/pull/99) 经guard与[CI](https://github.com/luohao0308/luohao-blog/actions/runs/38875014951)通过后合并为74411f3356a46d8f72c616a82ef74996fe7e299d，合并树0e5440934ceb9af2b9933d8a70df99c735c2084d与授权源码完全相同。真实MySQL/Redis步骤实际成功，不以被Skip的常规单测替代。

[自动release](https://github.com/luohao0308/luohao-blog/actions/runs/38875269631)成功，5项job全部通过；后端/前端镜像构建输出摘要、Registry摘要、manifest和config内容哈希及linux/amd64平台均核对一致。Git及自动release一次性授权均已消费，不能据此部署、迁移回填、修改凭据/平台、生成新预览或正式发布。用户明确正式Secret稍后保存，F2仍待前置条件。

## F2 当前停机点

正式Secret已由用户保存，元数据检查通过。用户针对744源码及固定镜像授权F2；源码/镜像和SQL、uploads、私密配置备份已准备验证。首次切换停止旧backend并检出744后，Git将两份公开tracked配置权限从0600改为0644，触发严格保留检查而中止；v16未启动，数据库只读复核为clean v15且新身份表不存在。原失败记录保留，单独恢复方案仅将两份准确Git blob恢复0600并执行尚未发生的启动/迁移/回填，不重复旧操作；待新的恢复授权。request域名提交遇管理员扫码，二维码过期，尚未确认保存。

F2停机点已通过用户针对该故障的一次恢复授权解决：恢复两份公开配置原权限后，仅执行未发生的v16启动与迁移/回填。数据库v16 clean，旧1条身份映射完整、账号与文章核心摘要保持，关闭微信时两接口412且普通读取正常；正式凭据仅更新wechat段后同v16后端重新健康，其他容器不变。request域名经管理员扫码并在平台新鲜页面确认成功；旧失败记录保留，不记作原apply成功。正式号首次绑定与真机仍待验收。

正式号一次手机预览已由准确744源码/miniprogram树0bfa180482d0038165f78f521679c48fb97e97bb生成并展示。保留原CLI包装结果，另以单次调用期间新生成的合法JPEG/时间窗/哈希与目检作为二维码证据，未重复生成。当前等待用户核对正式号名称、QQ站长首绑及再次登录、图片/阅读/筛选/搜索/收藏/AI引用；尚未声称真机全部通过，未上传/提审/发布。
