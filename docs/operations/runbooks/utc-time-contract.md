# UTC 时间契约切换与恢复

对应[批准计划](../../plans/UTC-TIME-CONTRACT-2026-11.md)与[操作工具](../../../backend/cmd/time-contract/README.md)。当前先完成本地验收；是否已生产切换以TASKS为准。此流程不提供生产权限，不包含凭据或一次性服务器快照。

## 时间与保护边界

应用连接显式使用UTC解析和UTC会话。历史修正的目标是保持当前Go/API时间意义，不推测每行历史事件真实发生时间。仅审核清单内13表26个TIMESTAMP列允许按固定28800秒转换；NULL与秒精度保持。DATETIME(6)检测时间、修订JSON、Redis/JWT、ES时间、正文/身份/角色/凭据版本/关联/计数/安全证据和公众号任务状态均受保护。

schema21只建立时间契约元数据结构。普通server、seed、reindex、content-release必须先通过外部ready状态与数据库marker检查，再允许普通迁移/业务或外部组件。旧20、未完成21、损坏/来源不明状态拒绝；没有自动回填或服务环境变量跳过路径。

`data.database.time_contract_state_dir`是正常持久目录配置，显式CLI `-state-dir`优先，随后配置，最后生产默认`/app/data/time-contract`。目录独立于数据库dump、用户uploads与配置恢复包，runtime只读挂载，受控operator才使用rw。它记录target、operation、kind、pending/ready及回执摘要，是流程状态而非授权凭据。

## 本地启动与已有数据库

新空库先显式运行`time-contract -mode fresh-init -operation-id <唯一标识> -writers-paused`，再启动server或运行seed。空库在迁移前识别，migration13产生的种子属于本次UTC初始化，不加八小时。本地公共配置使用`./data/time-contract`，从backend目录执行各CLI与server可读取同一目录。

已有非空库不能当空库初始化，不能仅凭schema20猜偏移。Operator要求完整InnoDB全局FK元数据可见性（PROCESS权限），不能把权限错误当零引用；视图、非事务表、未知对象以及目标时间列参与的任意入向/出向FK均拒绝，避免跨库副作用或关系重定向。目标数据库名称限定已审核的简单ASCII标识。计划模式验证原连接每个TIMESTAMP读取值与内部epoch的真实28800秒差，并绑定目标、业务结构和数据摘要；映射不符或混合状态停止。已有UTC测试库只由明确校验过本地Docker/context/label/loopback/随机数据库的testdata helper建立标记；这不是生产adopt或assume-UTC入口。

## 生产转换的受控顺序

1. 本地测试和独立review全部通过，准备准确源码、CI固定镜像、列清单、转换与维护窗口。取得本次Git交付及生产操作准确授权；所有旧授权不能复用。记录当前后端ID、镜像、完整Compose覆盖链、配置/资产摘要和schema状态。
2. 核对全写者清单并停止选中的后端及其worker；独立CLI、恢复任务等也不得并发写。核实原容器确实停止。现有watchdog对stopped不自动重启，但执行前须核对现场版本和行为，不以源码替代生产证据。
3. 在停写状态生成准确数据库备份与固定格式sidecar，并校验gzip、准确文件SHA和表清单。后端停止时不能使用`exec backend tar`，应使用获准固定镜像的只读uploads卷提取；独立时间状态目录不装进uploads包。备份表示legacy20内部epoch，不会自动修正时间。
4. 使用新固定镜像的SQL-only operator、原有正常配置loader和独立state卷生成canonical plan；stdout字节不重新格式化。核对准确manifest SHA。`backup-receipt`检查实际archive/sidecar、目标/op和当前before摘要。禁止只用一个自报SHA字符串代替备份证据。
5. 明确`apply`及准确manifest/备份文件、sidecar、typed receipt和停写确认。工具先写pending，再执行21元数据DDL，随后单事务完成白名单更新和marker；任何步骤失败保持停写。DDL与业务事务分开，21 clean但无完成marker仍是未完成状态。
6. COMMIT确认丢失只运行`status/reconcile`辨认before、after或unknown，不自动重放。只有原attempt、kind、target、manifest、备份身份及数据/保护摘要匹配，显式`activate`才可完成pending。错误、混合或无法判断状态保留证据另处理。
7. ready确认后才用同完整Compose链、固定镜像启动选中backend，保持state卷ro。核对健康、schema/marker、连接UTC、旧API时间与DB内部epoch一致、全部保护摘要/资产/其他容器保持。前端无需更新。不为此验收发邮件/正式微信/公众号、创建生产测试评论或重新上传小程序。

从21元数据DDL开始，就不能假设旧20镜像仍可启动。完成转换后使用兼容UTC契约的版本前滚；任何准确备份恢复或旧源码承接均需另行授权。不force版本、不删marker、不自动批量减回八小时。

## 备份与恢复

`backup.sh`采用固定单行mysqldump格式，从同一个准确dump推导表清单、逐表行数/数据和DDL摘要以及压缩/未压缩SHA；不把另一时刻实时SQL计数当同一快照。MySQL8.4回放可能给已有COLLATE的char/varchar列补冗余CHARACTER SET，helper只对此已证明的等价拼写规范化；实际字符集/排序规则和其余DDL仍必须相同。raw dump/archive SHA仍校验准确字节。

sidecar是完整性证据，不是SQL沙箱；只恢复来源明确、准确摘要获准的受控备份。不支持任意无sidecar历史SQL自动恢复。legacy20备份保持旧内部epoch，UTC21备份保持新内部epoch；都不会在dump/restore时自动加八小时。

恢复先校验输入包和sidecar，再以新随机attempt nonce写独立pending，随后停止并核实原backend。导入、同格式redump比对、实际DB canonical snapshot复验均成功后，绑定本attempt/nonce/target/backup/manifest的完整receipt才允许切ready并启动。旧receipt即使重复opID也不能清新pending。

导入失败、legacy20覆盖21留下旧marker、marker已导入但后续失败、数据/DDL或receipt不符，都保持后端停止；schema与marker存在本身不代表成功。legacy20恢复不得自动用UTC21服务启动或自动再次位移，需要准确来源与具体恢复方案。独立state目录不随DB/config/uploads恢复覆盖；手工重建容器也必须受同一pending门约束。

## 可重复本地验证

- `python3 scripts/test_time_contract.py`：真实legacy偏移/目标UTC基线、DT微秒/JSON/NULL/期限边界和错误恢复夹具。
- `python3 scripts/test_time_contract_integration.py`：MySQL状态机/并发/故障、真实Data与内容工具门、native CLI与实际MySQL的受控转换及恢复。仅服务生命周期用测试shim，不能把它说成真实生产Compose演练。
- `python3 -m unittest discover -s scripts -p 'test_time_*test.py'`以及明确的manifest/restore shell测试：本地Docker拒绝门、备份格式和恢复顺序。
- Go build/test/vet/lint、相关race与既有身份/文章修订真实MySQL回归；具体新鲜结果以受控验收输出为准。

所有fixture必须本地context、loopback及自有标签/随机数据库，不读取生产账号/令牌/正文。恢复临时dump与完整SQL诊断保持私密；只对外输出固定错误、摘要和计数。
