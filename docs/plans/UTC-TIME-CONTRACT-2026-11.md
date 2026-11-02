# UTC 时间契约与历史兼容修复计划

状态：approved。用户2026-11-03明确批准三步计划，先完成本地验证；生产另准确授权。日期：2026-11-03。风险：HIGH（历史时间回填、过期与调度判断、启动与恢复边界）。这是经App原生Architect与Critic顺序复核通过的批准计划，不是生产权限。

## 目标与验收不变量

统一新 MySQL 连接的解析与会话时间为 UTC，使应用返回的 epoch 与数据库 TIMESTAMP 内部 epoch 一致；对既有记录保留目前 API 所表达的时间意义。此次修复不推断、重写某个历史事件“本来应该发生”的真实时间。

1. 旧 API 时间的 RFC3339/epoch、空值、精度、排序和相同时间并列排序保持；新写入实际 UTC 时刻、SQL UNIX_TIMESTAMP(TIMESTAMP列)、API epoch 一致。
2. 正文、账号、密码/凭据版本、角色、关联、修订 JSON、计数、幂等键、安全证据、公众号任务状态均保持。前端继续东八区展示。
3. 只转换明确清单内 TIMESTAMP 列；DATETIME(6) lease/checked_at、JSON 时间、Redis/JWT epoch、ES 中现有 RFC3339 不偏移，不重建搜索或发送邮件/微信。
4. 邮件令牌截止、公众号可运行/恢复时刻、评论检测 lease/通过证据等在切换前后的语义一致（允许真实停机时间自然流逝）。没有实际邮件、微信、模型调用。
5. 普通应用启动、seed、reindex、content-release 的预检不得隐式进行历史时间回填；错误数据状态须先于任何业务写入或外部调用拒绝启动。
6. 首次回填和标记同一事务提交；已完成再次执行不再位移。提交结果未知先只读辨认状态，不自动重试。

## 已有证据与未知项

主代理已只读确认生产 main8a372ef/schema20，后端 /etc/localtime=Etc/UTC，驱动 parseTime=True/loc=Local；MySQL global/session SYSTEM、系统 CST，四个现有 blog 连接 time_zone 均 SYSTEM(+08)。已知文章发布操作14:50:31Z、API14:50:32Z、数据库 UNIX_TIMESTAMP(published_at)对应06:50:32Z；两个作品同差8小时。官方 MySQL TIMESTAMP 在存取时按会话时区转换，driver loc 不改变 MySQL 会话。

仓库证据：backend/internal/data/data.go:54 直接 sql.Open 后自动 Up；Ent schema/mixin.go:41使用time.Now。cmd/content-release/main.go:488另有直开连接且随后NewData；cmd/wechat-identity-backfill/main.go:51严格旧schema16入口；seed/reindex通过NewData。frontend/app/composables/useArticles.ts:120显式Asia/Shanghai；mini/utils/format.ts固定东八区。official_account_repo.go:166与193以Go时间参数判断可运行，account_email_token.go以Go时间参数比较截止。migration13用UTC_TIMESTAMP种入初始作品/About。

已知时间列范围（最终以审核过的schema指纹为准）：abouts、account_email_tokens(created/updated/expires)、article_revisions(created)、articles(created/updated/published)、categories、collections(saved)、comments、projects、subscribers、tags、users、wechat_identities(created)、wechat_sync_tasks(created/updated/next_attempt)均TIMESTAMP。wechat_comment_checks.lease_until/checked_at是DATETIME(6)。

主代理新鲜只读清单：13张含TIMESTAMP表、26列、69行，全部InnoDB；published_at有3个NULL、next_attempt_at有1个NULL；epoch0与加28800秒后越过TIMESTAMP上界的计数均0。各表行数为abouts1/account_email_tokens1/article_revisions4/articles12/categories0/collections0/comments4/projects6/subscribers0/tags32/users6/wechat_identities2/wechat_sync_tasks1。上线前需重新确认准确列名/类型/NULL/精度/默认/EXTRA/触发器、时间索引、记录量和范围，并拒绝未知时间列或新写入入口。无需读取或展示凭据和私有正文；用受控摘要、计数和公开时间样本验收。历史writer混杂不改变本轮“保持当前API含义”的目标，异常样本先列为独立历史事实问题。

## 选择与取舍

| 方案 | 收益 | 成本/风险 | 结论 |
|---|---|---|---|
| A：显式UTC连接，停写窗口内仅修TIMESTAMP内部epoch，保API含义 | DML可原子提交；类型/API不变；可验证旧语义与新时刻一致 | 需要时间契约标记、完整写入口门与受控恢复 | 推荐 |
| B：全部TIMESTAMP改DATETIME | 消除该类型的会话转换、扩大年月范围 | 多表DDL非整体原子，混合中间态；Ent类型契约也变；仍需UTC写入纪律 | 不采用 |
| C：仅修改loc或session/global TZ | 改动少 | 旧API时间、截止和调度可能立即移动8小时；混合写入加重 | 不采用 |

原则：保持现有业务语义；转换明确列；每步可观测且失败停写；生产行为与授权分开；不改全局MySQL或其他服务TZ。

## 三个有序切片

| 切片 | 结果及修改范围 | 依赖 | 验收 | 回退点 |
|---|---|---|---|---|
| S1 隔离复现与兼容基线 | 新 scripts/test_time_contract.py 及必要合成fixture/test；建立legacy+08/locUTC与目标UTC连接对照，产出列清单和时间/业务摘要 | 用户确认拆分；只读schema清单 | MySQL8.4真实驱动复现8小时相抵；同一固定时钟下令牌/任务/lease边界；备份恢复保原内部epoch；证明TIMESTAMP与DATETIME/JSON区别；合成状态矩阵与两种不完整恢复夹具 | 仅清理有唯一标签的本地fixture，无产品行为变更 |
| S2 UTC连接、受限转换工具与运行门 | backend/internal/data连接/契约模块，所有相关CLI，migration21元数据结构，新cmd/time-contract，针对性测试、Runbook、必要备份恢复入口；无新依赖 | S1基线通过 | 新物理连接/池扩容UTC；旧库普通启动拒绝；dryrun零写；明确apply一次转换；事务故障/未知commit/重复执行；状态矩阵全路径和backup hash/完整恢复回执负例；全量回归与独立review | 未apply前维持schema20与旧服务；本地可撤代码。生产回填后不自动回旧20 |
| S3 经准确授权的生产切换与只读验收 | 固定CI后端镜像、受控备份/停写/一次operator转换/启动新后端；前端和其他容器无需更新 | S2全部验收、备份恢复证明；新准确SHA/镜像/列清单/转换摘要/窗口授权 | schema/契约标记一致、准确备份hash及全表恢复验证回执齐备、旧API含义/非时间字段及证据保持，新旧epoch对齐，公开页面/权限/调度无外部调用；只更新选中backend | 开始前可保持原服务；提交后只兼容新契约前滚或针对准确备份另授权恢复，不执行反向批量加减时间 |

同时最多一个in_progress。此表确认仅允许按序本地实现/验证；生产切换仍需准确版本和操作授权。普通实现细节无需再次询问，只有接受标准/迁移范围/恢复边界实质变化才回到主代理。

## 批准实施契约

连接：统一使用现有mysql.Config解析DSN，保持凭据/网络/超时等其他设置；明确ParseTime=true、Loc=UTC、Params.time_zone对应'+00:00'。验证每个新物理连接实际session time_zone与固定时间解析；池内不得随意SET SESSION，UTC重建的连接保持同一契约。开发和运维模板同步说明，不能输出完整DSN。旧schema16 identity-backfill作为显式历史专用工具保留拒绝新schema，不让其成为绕过新生产契约的入口。

状态：新增schema21仅承载一个时间契约元数据表，不把业务回填放进普通migration.Up。服务与所有写能力CLI必须在自动Up和任何外部调用前执行状态门；content-release的直开SQL入口也必须先gate，再允许ES预检。schema16 identity-backfill保持严格历史专用语义，不新增服务器环境变量来绕过状态门。显式状态矩阵如下：

| 实际状态与来源证据 | 允许路径 | 普通启动/重复执行 |
|---|---|---|
| 迁移前无应用表的全新库 | 显式fresh-init，UTC连接执行迁移与migration13后续seed，offset=0，完成初始化标记 | 初始化前拒绝；完成后按UTC契约启动，不能把seed行误判成legacy数据 |
| 有可信创建/初始化记录的本任务自有UTC fixture | 显式fixture-bootstrap offset=0，核对fixture所有权、连接与来源证据 | 不按schema20猜offset；不能用服务端env跳过来源校验 |
| 已审核的生产legacy20与绑定的before摘要 | 显式legacy-convert，停写后按manifest只转换TIMESTAMP +28800秒 | 普通启动拒绝；只能走批准的operator路径 |
| 来源不明的非空库 | 拒绝；先补只读来源/当前API语义证据与独立计划 | 不自动选择0或28800，不因schema版本相同推断来源 |
| schema21 clean但无完成标记 | 仅原attempt及before摘要证明业务仍未转换时允许operator恢复；含DDL完成、DML未提交的中间态 | 普通启动拒绝；若不能证明仍是before则保持停写、只读对账 |
| schema21与完成标记匹配、契约一致 | UTC正常运行；相同manifest重复apply返回already-complete、零写 | 不再次位移；不同manifest或不一致摘要拒绝 |
| dirty、marker冲突、混合/部分恢复或无法判定状态 | 保持停写，只读对账与具体恢复方案 | 不force版本、不删marker、不自动反向位移 |

新元数据结构可在维护窗口独立DDL建立；若DDL后回填失败，21未标记是明确中间态，不能把“21 clean”当完成。恢复中的库还须满足下文完整导入回执门，schema/marker匹配本身不够。


Operator默认dryrun，只读核对绑定数据库、schema、旧session映射、表/列指纹、行数和合法范围，输出无敏感内容的review manifest。apply要求准确manifest摘要、备份回执和明确停写确认；只使用独立UTC连接，不启动应用/微信/邮件/ES组件。变更前重复预检并锁定目标行，确认全InnoDB、无未知触发器/ON UPDATE副作用、無外部写入。采用应用现有API解释作为目标：本次已证实的legacy+08/parseUTC组合，UTC连接读取的TIMESTAMP内部值按+28800秒得到目标。不得对DATETIME或JSON执行该公式。NULL、秒精度保持；预先拒绝越过TIMESTAMP上界或异常零日期。

在单一事务里更新白名单TIMESTAMP列并写入唯一完成标记，记录源契约、目标UTC、manifest摘要、转换计数/摘要。不更新业务版本号、不生成修订或发布任务，不应用Ent的自动updated_at钩子。转换完成再验证旧API时间=目标数据库epoch，同时确认排除字段摘要一致。同一marker再次apply返回已完成且零写；marker缺失/manifest不符/半恢复组合拒绝。COMMIT确认丢失时只读检查同一marker及摘要，不能只凭进程失败判回滚。

停写：冻结实际所有写入入口（backend及其worker、operator/seed/reindex/content-release、可能的恢复任务）。只停backend不等于全写者已隔离，需要操作清单和证据。不通过关闭全局服务或修改未授权凭据实现停写。现有公众号drafted任务保持；如果出现queued/ready/polling或中断任务，停止切换并交给主代理判定，不能为本修复发送消息或触发重试。

备份/恢复：mysqldump默认tz-utc保存内部epoch，错误epoch也会如实恢复，不能依赖dump修复。备份清单标明legacy20/UTC21、准确备份SHA256、表清单、逐表行数/摘要及marker摘要。恢复开始即保持停写并记录绑定该备份hash的restore attempt；只有全量导入命令成功、全表清单/行数/摘要与清单相符、schema/marker/epoch样本符合目标契约，才能生成同一attempt的完整成功回执并允许启动。旧回执、仅marker存在、或仅SQL进程退出不构成完整恢复证明。

必须拒绝两种故障：legacy20 dump恢复到已有21目标时遗留原UTC marker；UTC21 dump已写入marker但后续表导入失败。两者都保持停写，不能依赖schema/marker表看似正确而重启。deploy/restore.sh目前SQL失败仍重启后端，此任务只在相关恢复路径修正为失败保持停写，并增加绑定backup hash及完整导入验证回执；不拓展通用DR框架。成功转换后不直接使用旧20镜像；需要恢复旧备份时，须另获准确备份/目标/匹配源码授权，证明旧契约恢复完整，防恢复normalized数据后又重做+8。


## 失败预演与测试

1. 转换中进程退出/连接丢失：事务前/中/COMMIT后故障注入；业务列和marker应同时旧或同时新。未知确认只读对账，重复apply零位移；21元数据DDL与业务事务的中间态必须拒绝启动。
2. 混合写者或连接时区漂移：普通应用旧20启动、未标记21启动、并发operator、池扩容/重连、显式旧时区连接均有负例；有活跃写者或schema/manifest变化即停。应用认证cookie/JWT/Redis不随迁移延长。
3. 误移动排除字段或错误恢复：合成TIMESTAMP/DATETIME(6)/NULL/接近2038上界、revisionJSON RFC3339、已有微信双trace/hash、历史drafted任务；对排除字段逐字节摘要；分别恢复legacy20和UTC21，禁止对normalized备份再次偏移；注入legacy20覆盖已有21遗留marker、UTC21导入marker后失败，两者必须无完整成功回执并拒绝启动；恢复失败不重启。

S1/S2真实MySQL测试必须本地Unix Docker context、loopback、唯一容器/数据库/标签，清理只属己对象；不得导出生产账号/令牌/正文到本机fixture。使用合成历史数据，公开事件作为只读参照。固定时间下覆盖令牌正好过期前/后、任务due前/后和恢复cutoff、检测lease前/后，核对客户端日期/排序/CAS输入无变化。

验证入口：env -u GOROOT go build/test/vet ./...；golangci-lint；相关Go race；现有scripts/test_wechat_identity.py、test_article_revisions.py（适配显式UTC fixture初始化而保留原15→16和19→20历史演练）；新time-contract真实MySQL测试；frontend已有时间格式测试及必要跨日浏览器冒烟；backup/restore隔离演练。新测试入口在实现后登记实际命令，不虚报尚未存在脚本的通过结果。

## 交接与停止条件

主代理先取得Architect意见，再送Critic；争议解决后记录准确计划版本，再向用户呈现三切片。执行者按S1→S2顺序交付并独立review；主代理拥有最终验收及生产授权。当前S1/S2 completed（本地），S3 pending且未授权；生产保持原状态。

成功：生产实际UTC契约与旧API语义保持均有新鲜证据，保护字段/已有检测与外部任务保持，新固定版本和恢复边界已记录。任何未知列、非事务表、触发器、时间越界、writer未停、运行状态/marker不一致、摘要变化或提交结果未知均停止写入并只读对账；不得用强制版本、删除marker、自动反向偏移或修改历史外部证据掩盖失败。

权威参考：https://pkg.go.dev/github.com/go-sql-driver/mysql@v1.10.0#readme-loc ；https://dev.mysql.com/doc/refman/8.4/en/datetime.html ；https://dev.mysql.com/doc/refman/8.4/en/date-and-time-functions.html#function_unix-timestamp 。

## S1验收完成

自有loopback MySQL8.4真实driver复现offset28800，UTC修正保持旧Go/API时间；DATETIME微秒/JSON/NULL、6个固定时钟的过期/任务边界和lease相等边界通过。dump/restore保原内部epoch；legacy20覆盖已有21残留marker和marker后导入失败两夹具确认仅schema/marker不足。实际Go测试TestTimeBaselineRoundtrip零skip、两项remoteDocker拒绝测试、vet/lint0通过，自有容器清理。这里只验证基线；S2运行门/受控工具尚未实现。


## S2本地验收完成

已实现固定UTC连接、21元数据结构、独立state-dir、受限转换与恢复工具，以及实际server/seed/reindex/content-release路径的前置状态门。普通启动不自动修正历史数据，恢复nonce、完整回执、marker校验和显式unknown-commit对账均已落地。

9项真实MySQL必需测试零跳过：转换/故障/重放、空库种子初始化、并发operator、实际Data新写epoch、内容工具门先于ES、混合遗留marker、同名可更新VIEW，以及入向跨库级联和出向时间关系重定向。只允许BASE TABLE/InnoDB；无法完整检查全局InnoDB FK元数据时拒绝，任何目标时间列参与FK均拒绝。仅支持已审核的简单ASCII数据库标识。

真实native CLI与MySQL使用完整合成关系数据验证canonical plan、准确备份文件/sidecar回执、转换及重复零写、正确UTC恢复后才启动、marker后失败和legacy20遗留marker拒绝。Docker服务编排/执行载体为测试shim，SQL和CLI实际执行，不冒称生产Compose演练。MySQL8.4回放冗余CHARACTER SET显隐只作窄等价规范化，实际charset/collation、数据和raw dump/archive SHA仍严格核对。

root新鲜Go build/test/vet/lint0/race、前端92tests/lint、11项Python安全/备份/恢复测试及shell语法通过。既有身份/Redis/HTTP与文章CAS五项真实MySQL升级至21回归通过，可信UTC fixture由testdata专用helper显式初始化。独立实现复核APPROVE，对应源码集合5a8d59fb118e9ed8e95e2c7523aad90b6eb3f123309a826cf266f2378010b1d3；证据在本机output/time-contract，不进Git。

本轮无生产访问或数据修改，无远端Git、邮件/微信/公众号、搜索重建或小程序上传。S3仍需新准确源码/CI镜像/转换清单与停写窗口授权，不复用任何旧授权。
