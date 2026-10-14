# 作品集与关于内容维护

机器契约权威：`backend/api/blog/v1/project.proto`、`about.proto` 和生成HTTP/gRPC/OpenAPI。字段使用snake_case；本指南不替代proto。

Project公开读取为GET `/v1/projects/list`（projects/next_page_token）与GET `/v1/projects/{slug}`。公开/读者只可见published=true，不能通过published=false或分页token获得草稿；后台作者可查看全部并筛选上架状态。列表按sort asc,slug asc稳定排序。

后台POST `/v1/admin/projects` body直接Project；PUT `/v1/admin/projects/{slug}` body直接Project，update_mask选择字段，slug创建后不可变。DELETE同路径仅令published=false，保留数据，PUT可恢复上架。Reader/匿名不可维护。媒体URL只接受S2本服务路径或空，仓库/演示链接仅http/https，拒CRLF和可执行协议，不做任意URL下载。

About公开GET `/v1/about`返回单个对象；后台PUT `/v1/admin/about` body直接About、全量替换。技能组skills(group/items)、timeline(year/text)、contacts(label/href/text)、representative_links(label/href)，简介/当前方向/图片可编辑。联系链接可用http/https或简单mailto；不从私密账号配置导出联系方式。数据库abouts.singleton_key唯一main确保单例。

`000013_projects_about`新增projects和abouts，并用INSERT IGNORE保留原静态三项目与关于文案，不编造指标或经历。数据迁入后由后台管理，重复执行种子不能覆盖编辑；没有截图时如实显示空素材，不生成假项目证明。

升级/恢复验证：`python3 scripts/test_mysql_migrations.py --baseline 12 --target 13`、`--baseline 11 --target 13`（仅带指定label/volume/loopback端口的本任务隔离MySQL）。验证旧文章非默认封面/精选保留、3项项目/1个About main、排序种子、编辑后重复seed不覆写、真实SQL备份恢复重升级。回退保留新增表与图片，不运行down丢弃新内容；前端如需回退以明确版本执行，不把API失败静默伪装成旧静态成功。
