# 公共页面统一内容宽度

状态：本地验证通过，用户已要求上线；准备准确版本交付，等待精确版本一次性授权门禁。2026-10-20。

## 范围与验收

- 保留预览中导航1088px外框及Logo/头像位置；公共页面主内容外框1024px，桌面相对导航内容左右各内收32px。
- 导航与主内容分别使用`.site-nav-container`、`.site-content-container`；通过`main.css`共享变量维护导航最大宽度、内容内收及基础留白。768px起主内容相对导航两侧各内收32px；手机保持单层24px留白，避免重复收窄。
- 首页、关于（含顶部墨流）、文章/作品列表及详情、个人设置，以及分类、标签、归档、收藏、排行和账号页面沿用同一外框。后台管理布局不在范围内。
- 文章/作品详情及关于正文保留860px内部阅读列；账号表单保留自身窄列。关于动画、滚动吸附、页面功能与数据接口不改。
- 关于页移除重复水平padding，避免移动端两层留白；删除已失效的`wide`页面配置。
- 在320/390/768/1024/1440px验证导航位置保持、主内容居中、桌面两侧各内收32px、手机单层24px留白、详情阅读宽度、墨流正常运行且无横溢；包含已登录的个人设置。

## 基线、边界与回退

当前工作树基线为f221f99，包含#107墨流、#108阅读布局及#109个人设置。只读访问正式首页、关于和个人设置均返回200，可见首页动效标记、关于墨流和设置页根标记；公开页面不能证明精确部署SHA。

桌面目录`/Users/luohao/Desktop/vibecoding/luohao-blog`为另一个较旧版本，已有用户文档改动；本轮未写入该目录。所有变更仅在本任务worktree，未push、PR、merge或部署。撤销本轮6个前端文件及设置页E2E宽度断言差异即可恢复原布局。

## 前轮验证（1088px同宽版本，已被上方内收规则替代）

- `pnpm lint`、`pnpm typecheck`、`node --test tests/*.test.cjs`通过（62项）；最终68rem宽度的`pnpm build`通过。
- 生产构建在本机13108端口配合只读合成API fixture验收，50组路由/视口检查通过：320/390/1024/1440px统一居中外框和24px留白、860px阅读列、关于墨流active、无横溢/页面脚本异常。390/1440px另覆盖公共辅助页及模拟登录的个人设置。
- `scripts/e2e/settings_layout_e2e.cjs`同步1088/1040px断言并通过：分组、草稿/头像选择、键盘、深链、昵称重试、邮箱与密码/退出模拟流程、320/390/768px、暗色、长内容和横向页签。所有请求均为mock，无真实账号写入。
- `node --check scripts/e2e/settings_layout_e2e.cjs`、`git diff --check`通过；独立源码复核无阻塞问题。
- 本机证据：`/tmp/blog-container-evidence/report.json`、`/tmp/blog-container-evidence/about-{320,390,1024,1440}.png`、`/tmp/blog-container-evidence/settings/`；构建和设置回归日志为`/tmp/blog-container-build.log`、`/tmp/blog-container-settings.log`。浏览器检查脚本`/tmp/blog-container-check.cjs`。临时证据未提交。
- 本轮没有部署；正式版仍使用原宽度。后台管理采用独立布局，未改动。

## 当前内收版本验证

- 导航外框1088px、内部1040px；主内容外框1024px、内部976px。桌面主内容相对Logo与头像区域两侧各内收32px，手机保留原有单层24px留白。
- 本轮`pnpm lint`、最终1024px版本`pnpm build`、设置页E2E断言语法检查与`git diff --check`通过；无需重复未变的纯逻辑全量测试。
- 35组本地生产模式浏览器检查通过：320/390/768/1024/1440px的首页、关于墨流、文章/作品列表及详情、模拟登录设置页，验证导航位置、两侧内收、居中、860px阅读列、墨流active及无横溢/页面脚本异常。
- 设置页E2E断言更新为1024/976px，完整mock交互回归通过，未写入真实账号。
- 当前证据：`/tmp/blog-container-evidence/inset/report.json`及该目录下首页、关于、设置页多宽度截图；日志`/tmp/blog-container-inset-{build,browser,settings}.log`，针对性检查脚本`/tmp/blog-container-inset-check.cjs`。
- 本机生产预览`http://127.0.0.1:13108/`已替换并保持运行；仍沿用合成数据及模拟登录。没有部署或远端操作。

## 正式交付准备

用户于2026-10-20要求将本轮宽度变更上线makerhao.cn。范围限前端，保留后端、数据库、uploads及其他服务；不包含迁移或持久权限修改。远端main基线只读核对为f221f995cf657365fea5603a082c0843037e3daa，required CI为后端和前端两项，required review数量0。当前worktree缺少被忽略的本地workflow文件，已从同仓库主工作区原样恢复manifest与delivery_guard，不改变任何策略字段。后续远端动作仍需准确版本一次性授权、平台新鲜证据与guard放行。
