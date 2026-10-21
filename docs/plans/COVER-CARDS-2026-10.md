# 封面式卡片交付

状态：本地集成、78测试、lint/typecheck/build及浏览器组合回归完成，独立复核APPROVE；用户已选定视觉并请求上线，远端/生产操作待准确版本授权。

目标：将用户选定的全幅照片叠字/纯色大标题卡片上线，正常字号212px等高，桌面三列、手机单列。保留既有鼠标聚光/光边、悬停、图片缩放、箭头、卡片到真实详情的展开，以及键盘/减少动态效果降级。

范围：ArticleCard、ProjectCard、共享封面CSS、两列表grid与首页h2局部覆盖；开发预览复用真实组件。基于最新main ba709a0，不混入旧工作树账号/域名或临时输出。首页城市/关于背景、搜索筛选、API读取、已发表和精选规则原样保留。真实cover_url继续走assetUrl，静态种子只用于开发预览。

验证：前端全部现有Node测试、lint/typecheck/build；实际组件有图/无图、长文本/缺字段、亮暗/窄屏、图片失败、hover/聚光/焦点/标签及详情展开回归。

交付：feat/cover-cards → PR/required CI → main；请求仅替换生产frontend镜像，不重建backend/Caddy/存储，不改任何密钥或数据。旧frontend镜像不可变引用作为回退点；备份当前容器与compose部署输入，不输出env内容。生产检查makerhao.cn首页/文章/作品与资源、真实数据未变。

权限：本地manifest manual+ai；push/PR/merge须准确ref/SHA一次性授权及delivery_guard，生产按host/service/不可变版本/expiry逐次授权。历史grant不可复用。

独立复核补充：首页wrapper改Flex拉伸，正常有/无图212px，200%文字放大后同行338px且无裁剪。旧工作树的账号/域名记录与临时输出均未带入。
