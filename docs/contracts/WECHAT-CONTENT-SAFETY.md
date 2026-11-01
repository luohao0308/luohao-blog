# 小程序公开评论内容安全契约

状态：S1/S2已实现并完成隔离回归，真实MySQL与端到端验收在S3进行。关联[批准方案](../plans/MINIPROGRAM-CONTENT-SAFETY-2026-11.md)。

## 安全边界

- Web既有评论接口、审核与头像装饰保持原路径，不因为客户端自报来源触发或跳过微信检查。新版小程序完整切换专用评论接口；保留Web公开接口意味着无法阻止任意客户端读取其公开内容，本契约不声称这一点。
- 专用匿名读取只展示全站APPROVED、当前正式AppID的检测pass、输入指纹一致的评论。没有证据的旧评论不自动回填pass，也不回退Web列表。
- 评论display_name是提交时持久快照，后改账号昵称不回写旧评论。检测实际持久化的昵称快照（资料场景1）和正文（评论场景2），两者均pass才具备小程序展示资格。动态Web头像不进入小程序投影，保留既有首字回退；本轮不扩张到用户媒体检测。
- 已有通过证据的读取不依赖新的上游请求；停用检测、Redis或微信故障都不能把未知状态升级为pass。

## 身份

首次创建/实际重检先验证JWT，再接收一次新的wx.login code；同键既有创建结果的本人读取不重复消费code。服务端用固定配置AppID调用Code2Session并查询现有(AppID,openid)身份，必须与JWT UserID相同。不接受客户端openid/AppID，不新建绑定、不回退旧openid列、不使用管理员身份代替作者。

身份缺失/微信code失效返回412，账号错配返回403；只有JWT缺失/失效在消耗code前使用401。禁止在消耗单次code后因业务身份错误返回401，引发小程序请求层重放原POST。微信近两小时条件仍以上游返回为准，fresh code不构成豁免。

## 服务端检测客户端（S1）

- `ContentSafetyChecker`只接受内部OpenID、Content和场景1/2，返回pass/review/risky及追踪信息。配置与明确开关齐备才允许调用；S1不接生产业务、不默认启用。
- [stable_token](https://developers.weixin.qq.com/miniprogram/dev/server/API/mp-access-token/api_getstableaccesstoken.html)：服务端POST，grant_type/client_credential、appid/secret、force_refresh=false；小程序安全令牌使用独立AppID命名空间，TTL不超过响应剩余expires_in，不能固定续满7200秒或强刷公众号令牌。
- [msgSecCheck](https://developers.weixin.qq.com/miniprogram/dev/server/API/sec-center/sec-check/api_msgseccheck.html)：服务端POST，version=2，content/scene/openid，令牌只在服务端请求中使用。综合suggest决定结果；缺字段、畸形/超大响应、身份过期、额度/网络错误一律不是pass。
- 超时、取消、限长和拒绝重定向均在客户端边界处理。无效令牌最多清除与本次令牌仍一致的缓存，本实现不自动重放检测请求。61010是访问身份过期，不是刷新服务令牌可以解决的问题。
- 不输出上游URL、errmsg、OpenID、正文、Secret或令牌。错误reason使用固定API枚举；正式请求与相应数据传输另按准确版本授权。

## S2接口及持久化要求

已新增专用公开列表、已认证提交、作者本人列表与作者重检接口（MiniCommentService）：GET /v1/mini/articles/{slug}/comments；POST /v1/mini/comments；GET /v1/mini/articles/{slug}/comments/mine；POST /v1/mini/comments/{id}/recheck。DTO不携带OpenID/AppID；私有列表仅按JWT账号读取。公开投影不装饰未经媒体检测的头像。

提交必须先原子创建PENDING评论和检测记录，随后对该实际快照发起两个场景检查，网络调用不得占用SQL事务。不能先检测再调用会重新读取昵称并自动公开的旧Submit。仅两场景pass、原有本地审核允许且限流健康时，初次检查才能自动批准；否则保留原有人工审核门禁。

检测记录按(comment_id,app_id)唯一，绑定不可变identity_id、输入哈希、状态、generation、lease、两trace及检查时间。输入哈希采用版本化结构，包含评论ID/作者ID/AppID/identityID/文章slug/存储昵称/正文。完成时核对当前代、哈希、所有权和评论仍存在；晚到结果不能覆盖新代，删除后不得upsert复活。删除评论清除检测证据、身份引用和trace；若存在创建幂等键，则仅保留不含正文/昵称/OpenID的deleted tombstone及创建负载哈希，阻止同键网络重放重建评论。

作者重检要求新微信code、所有权、限流及恢复状态条件；相同pass快照不重复请求，不对不变risky/review内容反复碰运气。重检默认只改检测证据，不自动越过站内审批。管理员Web批准不能直接写微信pass。

分页先在SQL中过滤全站审批、AppID及检测状态；若再校验哈希，游标按扫描位置推进，不能以过滤后的条数判定已到末尾。小程序响应必须明确visible和检测状态，不能仅凭全站APPROVED显示“已发布”；退出/换账号清空作者私有状态。初次提交使用(app_id,author_id,client_request_id)唯一键，创建负载哈希只绑定规范化slug/content和协议版本；同键同负载返回原快照、不重复检查，同键异负载409。并发冲突整笔回滚并读取赢家；结果未知不能自动更换键。已删除键返回固定已删除结果，不重建评论。既有Web评论重检的request_id为NULL，避免空字符串唯一冲突。客户端稳定保存本次提交意图，换账号隔离；删除后的幂等元数据不保存原文，不承诺所有外部调用exactly-once。

## 必须保留的验证

两场景pass/review/risky及上游异常；不同AppID/账号错配和无身份；检测期间改昵称、审批和删除；旧代pass晚到；lease恢复；未审历史/网站评论不进入mini；管理员批准不绕过；失效哈希分页；作者跨账号读取拒绝；所有错误/日志不泄漏凭据。固定HTTP/Redis及真实数据库测试使用合成数据，不联系正式微信接口。

## 接线、消费者与恢复

- 新增迁移19及wechat_comment_checks表，不回填旧内容为pass。author删除级联清理，identity删除置空使旧证据失效；comment删除清正文和检测凭据，最小创建tombstone继续阻止同键重建。
- wechat.content_safety_enabled默认false；正式启用还要求既有WeChat登录配置完整、Redis地址明确、数据库debug关闭。专用Redis池启用ContextTimeoutEnabled、独立清理和有限I/O，不修改网站共享Redis选项；限流仍沿用既有账号/IP预算键。
- 返回MiniCommentRecord包含comment、数字safety_state、visible、can_recheck和固定safety_error；数字状态0/1/2/3/4/5/6分别为unknown/checking/pass/review/risky/unavailable/deleted，不返回身份、hash、trace或令牌。公开输出头像为空。已通过态若缺必要双trace或指纹失效，会降为unknown并允许重新检测。
- 错误码87014作为确定风险终态，不转换成鼓励重试的服务故障；负向风险没有成功trace也不能成为pass。缺失通过态trace仍不放行。
- 小程序全量切换四条专用路径，不回退Web；本人项按账号/文章与公开项合并、去重并使用visible判断。创建请求键按账号/文章/规范化正文保存，网络失败保留；新body或明确成功才更新意图，删除storage时匹配已确认requestID。新存储不包含微信code或Token。
- 私有异步调用绑定账号与身份epoch：首次发送、刷新前后和重放前后检查；普通同账号token轮换不更改epoch，退出/切账号/重新登录使旧调用失效。刷新失败同步清除私有UI，卸载失效全部相关回调，不让旧成功清掉新页意图。
- 检测业务总截止20秒；小程序创建/重检超时25秒并传递同一身份guard。每次服务端检测调用5秒上界，依次使用资料和评论场景，不在SQL事务中等待微信。正常错误保存待检结果，已持久化后检测确认丢失仍返回可找回的receipt；明确删除返回410而不回显旧待审对象。
- 小程序本地保存草稿/提交凭据及向微信发送公开昵称与评论文本的用途需在正式联调前与隐私内容核对；不从“已更新/已声明”推断这些新增数据流已获说明。0.1.0旧开发包未切换安全路径，不作为本实现完成后的提审候选。
