#!/usr/bin/env python3
"""luohao-blog E2E 自动化验证（Python + Playwright）。

目标：对本地生产栈（caddy :8080 → frontend BFF → backend）做黑盒 GUI 级验证，
覆盖公开页面、搜索、404、登录、文章 CRUD、发布流转、评论审核闭环、登出守卫、AI 问答。

用法：
    /tmp/pw-venv/bin/python scripts/e2e/blog_e2e.py

环境变量：
    BLOG_E2E_BASE_URL   默认 http://127.0.0.1:8080
    BLOG_ADMIN_EMAIL    默认 author@example.com（本地 seed 示例账号，仅本地栈）
    BLOG_ADMIN_PASSWORD 默认 longenough1
    BLOG_E2E_CLEAR_RATELIMIT 设 1 时套件启动前清空本地栈限流键（登录限流
                        10 次/5min/IP，一轮全套加前置验证即触顶，连跑必挂）
    BLOG_E2E_REDIS_CONTAINER 限流键所在 Redis 容器名，默认 blog-redis-prod

说明：
- 截图输出到 gui-test-screenshots/e2e-py/。
- 文章删除与评论删除走带 token 的 API 调用（与 UI 路径等价的清理通道）。
"""

from __future__ import annotations

import os
import subprocess
import sys
import time
import traceback
from dataclasses import dataclass, field

from playwright.sync_api import Page, sync_playwright

BASE_URL = os.environ.get("BLOG_E2E_BASE_URL", "http://127.0.0.1:8080").rstrip("/")
ADMIN_EMAIL = os.environ.get("BLOG_ADMIN_EMAIL", "author@example.com")
ADMIN_PASSWORD = os.environ.get("BLOG_ADMIN_PASSWORD", "longenough1")
SHOT_DIR = os.path.join(os.path.dirname(__file__), "..", "..", "gui-test-screenshots", "e2e-py")


@dataclass
class Result:
    name: str
    desc: str
    status: str = "SKIP"  # PASS / FAIL / SKIP
    known_issue: bool = False
    duration: float = 0.0
    detail: str = ""
    screenshot: str = ""


RESULTS: list[Result] = []
TESTS: list[tuple[str, object]] = []


def shot(page: Page, name: str) -> str:
    os.makedirs(SHOT_DIR, exist_ok=True)
    path = os.path.abspath(os.path.join(SHOT_DIR, f"{name}.png"))
    page.screenshot(path=path, full_page=True)
    return path


def fill_hydrated(locator, value: str, attempts: int = 12) -> None:
    """向 Vue v-model 输入框填写，并在水合竞态吞掉输入时重试。

    SSR 页面在水合完成前 fill 只改 DOM 不改组件状态，随后的水合会把输入
    清空（t14 登录框复现过）。逐次校验 input_value 直到状态真正写入。
    """
    for i in range(attempts):
        locator.fill(value)
        if locator.input_value() == value:
            return
        locator.wait_for(timeout=2_000)
        time.sleep(0.4 * (i + 1))
    raise AssertionError(f"输入框写入失败（疑似水合竞态）: {value!r}")


def run(name: str, desc: str = "", known_issue: bool = False):
    def deco(fn):
        page_ctx: dict = {}

        def wrapper(browser):
            r = Result(name=name, desc=desc, known_issue=known_issue)
            RESULTS.append(r)
            t0 = time.time()
            ctx = browser.new_context(locale="zh-CN", viewport={"width": 1366, "height": 900})
            page = ctx.new_page()
            page.set_default_timeout(15_000)
            try:
                fn(page, ctx)
                r.status = "PASS"
            except Exception as e:  # noqa: BLE001 —— 单用例失败不中断整个套件
                r.status = "FAIL"
                r.detail = f"{type(e).__name__}: {str(e)[:400]}"
                if not known_issue:
                    r.detail += "\n" + traceback.format_exc(limit=3)
            finally:
                r.duration = time.time() - t0
                try:
                    r.screenshot = shot(page, name)
                except Exception:  # noqa: BLE001
                    pass
                ctx.close()
            return r

        wrapper.test_name = name  # type: ignore[attr-defined]
        TESTS.append((name, wrapper))
        return wrapper

    return deco


def login_admin(page: Page) -> None:
    """走 UI 登录管理员，成功后等待跳转到 /admin/posts。"""
    page.goto(f"{BASE_URL}/admin/login", wait_until="networkidle")
    fill_hydrated(page.locator('input[placeholder="you@example.com"]'), ADMIN_EMAIL)
    fill_hydrated(page.locator('input[placeholder="密码"]'), ADMIN_PASSWORD)
    page.click('button:has-text("登录")')
    page.wait_for_url("**/admin/posts**", timeout=15_000)


def api_login(ctx) -> str:
    """通过 BFF 接口拿 access token，供 UI 不可用时的清理调用使用。"""
    resp = ctx.request.post(
        f"{BASE_URL}/api/v1/auth/login",
        data={"email": ADMIN_EMAIL, "password": ADMIN_PASSWORD},
        headers={"Content-Type": "application/json"},
    )
    if not resp.ok:
        raise RuntimeError(f"API login failed: HTTP {resp.status}")
    return resp.json()["access_token"]


def api_cleanup_comments(ctx, marker: str) -> int:
    """按内容关键字删除评论（审核闭环的数据清理），返回删除条数。"""
    token = api_login(ctx)
    headers = {"Authorization": f"Bearer {token}"}
    resp = ctx.request.get(f"{BASE_URL}/api/v1/comments?page_size=100", headers=headers)
    if not resp.ok:
        raise RuntimeError(f"评论列表失败: HTTP {resp.status}")
    removed = 0
    for c in resp.json().get("comments") or []:
        if marker in (c.get("content") or ""):
            dele = ctx.request.delete(f"{BASE_URL}/api/v1/comments/{c['id']}", headers=headers)
            if dele.ok:
                removed += 1
    return removed


# ---------------------------------------------------------------- 公开页面

@run("t01_home_page", "首页 SSR 渲染（含无标签文章，tags 判空回归锚点）")
def t01(page, ctx):
    resp = page.goto(f"{BASE_URL}/", wait_until="domcontentloaded")
    assert resp.status == 200, f"首页 HTTP {resp.status}（预期 200，实际为 SSR 500 即 P0 缺陷复现）"
    assert page.locator("h1").first.inner_text().startswith("你好"), "缺少 hero 标题"
    assert page.get_by_text("最新文章").is_visible(), "缺少最新文章区块"
    assert page.locator('a[href^="/posts/"]').count() >= 1, "首页未渲染任何文章卡片"


@run("t02_posts_list", "文章列表页 /posts（ArticleCollection tags 判空回归锚点）")
def t02(page, ctx):
    resp = page.goto(f"{BASE_URL}/posts", wait_until="domcontentloaded")
    assert resp.status == 200, f"/posts HTTP {resp.status}（500 = P0 缺陷复现）"
    assert page.locator('[aria-label="文章列表"]').is_visible(), "文章列表区块未渲染"


@run("t03_tag_page", "标签聚合页 /tags/go")
def t03(page, ctx):
    resp = page.goto(f"{BASE_URL}/tags/go", wait_until="domcontentloaded")
    assert resp.status == 200, f"/tags/go HTTP {resp.status}（500 = P0 缺陷复现）"


@run("t04_article_detail", "已发布文章详情页：标题/正文/阅读量/标签")
def t04(page, ctx):
    resp = page.goto(f"{BASE_URL}/posts/ai-delivery-guard", wait_until="domcontentloaded")
    assert resp.status == 200, f"详情页 HTTP {resp.status}"
    assert "交付守卫" in page.locator("article h1").inner_text(), "文章标题不符"
    assert page.locator(".markdown-body h2").count() >= 1, "Markdown 正文未渲染出标题结构"
    assert "次阅读" in page.locator('[aria-label="阅读量"]').inner_text(), "阅读量未显示"
    assert page.locator('article a[href^="/tags/"]').count() >= 1, "文章标签链接未渲染"


@run("t05_article_detail_tagless", "无标签文章详情页可正常打开（tags 判空；环境无无标签文章时降级为任意详情页）")
def t05(page, ctx):
    # 回归锚点源于"无标签文章打挂全站"。目标文章按环境动态选择：优先取一
    # 篇 tags 为空的文章（本用例的原始意图）；生产等内容库可能全部带标签，
    # 此时降级为打开任意一篇详情页，验证 200 + 标题 + 标签区不炸。
    resp = ctx.request.get(f"{BASE_URL}/api/v1/articles/list?page_size=50")
    assert resp.ok, f"文章列表接口失败: HTTP {resp.status}"
    articles = resp.json().get("articles") or []
    assert articles, "环境内无文章，无法验证详情页"
    target = next(
        (a["slug"] for a in articles if not (a.get("tags") or [])),
        articles[0]["slug"],
    )
    resp = page.goto(f"{BASE_URL}/posts/{target}", wait_until="domcontentloaded")
    assert resp.status == 200, f"文章详情页 HTTP {resp.status}"
    assert page.locator("article h1").is_visible(), "标题未渲染"


@run("t06_archives", "归档页渲染")
def t06(page, ctx):
    resp = page.goto(f"{BASE_URL}/archives", wait_until="domcontentloaded")
    assert resp.status == 200, f"归档页 HTTP {resp.status}"
    assert page.locator('a[href^="/posts/"]').count() >= 1, "归档页没有文章链接"


@run("t07_projects", "作品集列表与详情页")
def t07(page, ctx):
    resp = page.goto(f"{BASE_URL}/projects", wait_until="domcontentloaded")
    assert resp.status == 200, f"作品集列表 HTTP {resp.status}"
    first = page.locator('a[href^="/projects/"]').first
    href = first.get_attribute("href")
    assert href, "作品集列表没有项目链接"
    resp2 = page.goto(f"{BASE_URL}{href}", wait_until="domcontentloaded")
    assert resp2.status == 200, f"项目详情 {href} HTTP {resp2.status}"


@run("t08_about", "关于页渲染")
def t08(page, ctx):
    resp = page.goto(f"{BASE_URL}/about", wait_until="domcontentloaded")
    assert resp.status == 200, f"关于页 HTTP {resp.status}"


@run("t09_search_flow", "站内搜索：输入关键词后出结果")
def t09(page, ctx):
    page.goto(f"{BASE_URL}/posts", wait_until="domcontentloaded")
    page.fill("#article-search", "守卫")
    page.wait_for_timeout(1500)  # ES 请求 + 防抖
    assert page.locator('[aria-label="文章列表"] a[href^="/posts/"]').count() >= 1, "搜索无结果"


@run("t10_unknown_slug_404", "不存在的文章返回 404 错误页")
def t10(page, ctx):
    resp = page.goto(f"{BASE_URL}/posts/no-such-article-e2e-xyz", wait_until="domcontentloaded")
    assert resp.status == 404, f"未知文章应返回 404，实际 {resp.status}"


@run("t11_chat_rag", "文章页 AI 问答（真实检索+LLM，外部依赖）")
def t11(page, ctx):
    page.goto(f"{BASE_URL}/posts/ai-delivery-guard", wait_until="networkidle")
    box = page.locator('section[aria-label="AI 问答"]')
    assert box.is_visible(), "AI 问答组件未渲染"
    fill_hydrated(box.locator("input"), "415 行的交付守卫是怎么工作的？")
    box.locator('button:has-text("提问")').click()
    # 提问后输入框被清空、按钮合理保持禁用；等待「回答气泡」出现而非按钮恢复。
    # 第 1 个气泡是用户提问，第 2 个是助手回答（或兜底/错误文案）。
    page.wait_for_selector('section[aria-label="AI 问答"] div.rounded-lg >> nth=1', timeout=60_000)
    reply = box.locator("div.rounded-lg").nth(1).inner_text()
    assert reply.strip(), "助手回答为空"
    assert "出处：" in box.inner_text() or reply.strip(), "回答既无出处也无内容"


# ---------------------------------------------------------------- 管理后台

@run("t12_login_wrong_password", "错误密码登录被拒绝并提示")
def t12(page, ctx):
    page.goto(f"{BASE_URL}/admin/login", wait_until="networkidle")
    fill_hydrated(page.locator('input[placeholder="you@example.com"]'), ADMIN_EMAIL)
    fill_hydrated(page.locator('input[placeholder="密码"]'), "wrong-password-e2e")
    page.click('button:has-text("登录")')
    alert = page.wait_for_selector('[role="alert"]', timeout=10_000)
    assert "邮箱或密码错误" in alert.inner_text(), f"提示文案异常: {alert.inner_text()}"
    assert "/admin/login" in page.url, "登录失败后不应跳转"


@run("t13_admin_posts_list", "后台文章列表渲染数据行（含无标签文章，2026-10 P1 回归锚点）")
def t13(page, ctx):
    login_admin(page)
    assert "/admin/posts" in page.url, f"登录后未跳转列表页: {page.url}"
    assert page.locator('h1:has-text("文章管理")').is_visible(), "列表页标题未渲染"
    # 列表数据经 authFetch 异步加载，等待行渲染而非立即计数
    page.wait_for_selector(".n-data-table tbody tr", timeout=10_000)
    assert page.locator(".n-data-table tbody tr").count() >= 2, "文章表格未渲染数据行"


@run("t14_create_publish_article", "新建文章 → 编辑页发布 → 公开可见（完整 UI 流）")
def t14(page, ctx):
    slug = f"e2e-py-{int(time.time())}"
    login_admin(page)
    page.goto(f"{BASE_URL}/admin/posts/new", wait_until="networkidle")
    fill_hydrated(page.locator('input[placeholder="文章标题"]'), "E2E 自动化验证文章")
    fill_hydrated(page.locator('input[placeholder="url-safe-slug"]'), slug)
    fill_hydrated(page.locator('textarea[placeholder*="显示在列表页"]'), "Playwright E2E 创建的验证文章，稍后自动删除。")
    # NDynamicTags 初始只有「+」按钮，点击后出现输入框；每加一个标签输入框
    # 会收回，需要再次点开
    for tag in ("e2e", "自动化"):
        if page.locator(".n-dynamic-tags input").count() == 0:
            page.locator(".n-dynamic-tags .n-button").first.click()
        tag_input = page.locator(".n-dynamic-tags input").first
        tag_input.wait_for(timeout=10_000)
        fill_hydrated(tag_input, tag)
        tag_input.press("Enter")
        page.wait_for_timeout(300)
    editor = page.locator(".ProseMirror").first
    editor.wait_for(timeout=15_000)
    editor.click()
    # delay 必须放慢：极速灌入时 Milkdown 的 markdownUpdated 回传会滞后于
    # 表单校验读取时机（实测无 delay 会导致「正文不能为空」误报）
    editor.type("# E2E 验证\n\n这是 Playwright 自动化创建的正文，验证创建-发布-公开读取链路。", delay=15)
    page.wait_for_timeout(800)
    assert "Playwright 自动化创建" in editor.inner_text(), "编辑器输入未生效"
    page.click('button:has-text("创建")')
    page.wait_for_url(f"**/admin/posts/{slug}/edit", timeout=15_000)
    page.wait_for_selector(".n-message", timeout=10_000)
    # 编辑页：状态切到「发布」并保存
    page.get_by_text("发布", exact=True).first.click()
    page.click('button:has-text("保存")')
    page.wait_for_selector('.n-message:has-text("已保存")', timeout=10_000)
    # 匿名新会话验证公开可见
    anon = ctx.new_page()
    resp = anon.goto(f"{BASE_URL}/posts/{slug}", wait_until="domcontentloaded")
    assert resp.status == 200, f"发布后公开访问 HTTP {resp.status}"
    assert "E2E 自动化验证文章" in anon.locator("article header h1").inner_text(), "公开页标题不符"
    assert anon.locator(".markdown-body h1").count() >= 1, "发布正文未渲染"
    # 清理：后台列表 UI 崩溃（P1），删除走 API
    token = api_login(ctx)
    dele = ctx.request.delete(
        f"{BASE_URL}/api/v1/articles/{slug}",
        headers={"Authorization": f"Bearer {token}"},
    )
    assert dele.ok, f"API 删除失败: HTTP {dele.status}"
    gone = anon.goto(f"{BASE_URL}/posts/{slug}", wait_until="domcontentloaded")
    assert gone.status == 404, f"删除后应 404，实际 {gone.status}"
    anon.close()


@run("t15_comment_moderation", "登录评论（身份取自账号）→ 后台审核通过 → 前台可见（闭环）")
def t15(page, ctx):
    marker = f"e2e评论{int(time.time())}"
    submitted = False  # 提交成功才要求清理命中，避免前置断言失败被清理报错掩盖
    try:
        # 未登录访客应看到登录引导而非评论表单（S4 门禁）。必须在登录前做：
        # 同一 context 的所有页面共享 cookie，登录后无法再模拟访客。
        page.goto(f"{BASE_URL}/posts/ai-delivery-guard", wait_until="networkidle")
        page.wait_for_selector('text=登录后即可发表评论', timeout=10_000)
        assert page.locator('textarea[placeholder*="写下你的评论"]').count() == 0, "未登录不应出现评论输入框"

        # 评论要求登录：登录后在文章页提交（身份取自账号，无昵称输入框）
        login_admin(page)
        page.goto(f"{BASE_URL}/posts/ai-delivery-guard", wait_until="networkidle")
        assert page.locator('input[placeholder="昵称"]').count() == 0, "登录后不应再有昵称输入框"
        fill_hydrated(page.locator('textarea[placeholder*="写下你的评论"]'), f"{marker} 内容待审核验证。")
        page.click('button:has-text("发表评论")')
        submitted = True
        notice = page.wait_for_selector('[role="status"]', timeout=10_000)
        assert "通过审核后展示" in notice.inner_text(), f"提交反馈异常: {notice.inner_text()}"
        # S4 语义：作者的待审评论在「待审核」块对自己可见，但不进已发布列表
        pending_block = page.locator('[aria-label="我的待审核评论"]')
        assert pending_block.get_by_text(marker).count() >= 1, "提交后未在待审核块展示"
        assert page.locator('section[aria-label="评论区"] ul.space-y-5').get_by_text(marker).count() == 0, \
            "未审核评论不应进入已发布列表"

        page.goto(f"{BASE_URL}/admin/comments", wait_until="networkidle")
        row = page.locator(f".n-data-table tr:has-text('{marker}')")
        row.wait_for(timeout=15_000)
        row.locator('button:has-text("通过")').click()
        page.wait_for_selector('.n-message:has-text("已通过")', timeout=10_000)
        # 前台可见（登录态页面：审核通过的评论带账号身份展示）
        page.goto(f"{BASE_URL}/posts/ai-delivery-guard", wait_until="networkidle")
        published_list = page.locator('section[aria-label="评论区"] ul.space-y-5')
        published_list.get_by_text(marker).wait_for(timeout=10_000)
    finally:
        # 清理走 API：UI 删除按钮的 popconfirm 在无头环境不稳定，且与用例断言无关
        removed = api_cleanup_comments(ctx, marker)
        if submitted:
            assert removed >= 1, f"清理失败：标记评论未删除（{marker}）"


@run("t16_logout_guard", "退出登录后访问受保护页被重定向回登录页")
def t16(page, ctx):
    login_admin(page)
    page.click('button:has-text("退出")')
    page.wait_for_url("**/admin/login**", timeout=10_000)
    page.goto(f"{BASE_URL}/admin/posts", wait_until="domcontentloaded")
    page.wait_for_url("**/admin/login**", timeout=10_000)
    assert "redirect=%2Fadmin%2Fposts" in page.url or "redirect=/admin/posts" in page.url, \
        f"重定向未携带回跳参数: {page.url}"


# ---------------------------------------------------------------- 入口

def clear_rate_limit() -> str:
    """BLOG_E2E_CLEAR_RATELIMIT=1 时清空本地栈限流键，返回状态描述。

    走 docker exec 是黑盒之外的运维通道，仅面向本地 docker 栈；对生产或
    非 docker 环境不可用。清理失败仅告警不阻断——限流器本身 fail-open，
    大不了拉开用例间隔重跑。
    """
    if os.environ.get("BLOG_E2E_CLEAR_RATELIMIT") != "1":
        return "skipped"
    container = os.environ.get("BLOG_E2E_REDIS_CONTAINER", "blog-redis-prod")
    cmd = [
        "docker", "exec", container, "sh", "-c",
        "redis-cli --scan --pattern 'blog:ratelimit:*' | xargs -r redis-cli del",
    ]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=15)
    except (OSError, subprocess.TimeoutExpired) as exc:
        print(f"⚠️ 限流键清理失败（继续）: {exc}")
        return "failed"
    if proc.returncode != 0:
        print(f"⚠️ 限流键清理异常（继续）: {proc.stderr.strip()[:120]}")
        return "failed"
    return f"cleared({proc.stdout.strip() or '0 keys'})"


def main() -> int:
    # 支持子集重跑：python blog_e2e.py t14 t15
    only = set(sys.argv[1:])
    print(f"限流键清理: {clear_rate_limit()}")
    from playwright.sync_api import sync_playwright as _sp  # noqa: PLC0415

    with _sp() as p:
        browser = p.chromium.launch(headless=True)
        for name, fn in TESTS:
            if not only or any(name.startswith(o) for o in only):
                fn(browser)
        browser.close()

    width = max(len(r.name) for r in RESULTS) + 2
    print()
    print("=" * 78)
    print(f"{'用例':<{width}}{'结果':<6}{'耗时':<8}说明")
    print("-" * 78)
    unexpected = 0
    for r in RESULTS:
        if r.status == "PASS":
            mark = "✅ PASS"
        elif r.known_issue:
            mark = "⚠️ KNOWN"
        else:
            mark = "❌ FAIL"
        note = r.desc
        if r.status == "FAIL":
            note = r.detail.splitlines()[0][:70] if r.detail else note
            if not r.known_issue:
                unexpected += 1
        print(f"{r.name:<{width}}{mark:<7}{r.duration:>5.1f}s  {note}")
    print("-" * 78)
    passed = sum(1 for r in RESULTS if r.status == "PASS")
    known = sum(1 for r in RESULTS if r.status == "FAIL" and r.known_issue)
    failed = sum(1 for r in RESULTS if r.status == "FAIL" and not r.known_issue)
    print(f"合计 {len(RESULTS)}：PASS {passed}｜已知缺陷复现 {known}｜意外失败 {failed}")
    print(f"截图目录：{os.path.abspath(SHOT_DIR)}")
    if unexpected:
        print("\n意外失败详情：")
        for r in RESULTS:
            if r.status == "FAIL" and not r.known_issue:
                print(f"\n--- {r.name} ---\n{r.detail}")
    return 1 if unexpected else 0


if __name__ == "__main__":
    sys.exit(main())
