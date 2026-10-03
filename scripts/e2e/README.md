# E2E 自动化验证（Python + Playwright）

对本地生产栈（`deploy/compose.prod.yml`，caddy :8080）做黑盒 GUI 级验证的黑盒套件。

## 运行

```bash
# 一次性准备（PEP 668 下不要直接 pip install 到系统 Python）
python3 -m venv /tmp/pw-venv
/tmp/pw-venv/bin/pip install playwright
/tmp/pw-venv/bin/playwright install chromium

# 全量跑（需本地生产栈在 :8080 运行）
/tmp/pw-venv/bin/python scripts/e2e/blog_e2e.py

# 子集重跑（前缀匹配）
/tmp/pw-venv/bin/python scripts/e2e/blog_e2e.py t14 t15
```

- 默认目标 `http://127.0.0.1:8080`，可用 `BLOG_E2E_BASE_URL` 覆盖。
- 管理员凭据默认用本地 seed 示例账号（`author@example.com` / `longenough1`），
  可用 `BLOG_ADMIN_EMAIL` / `BLOG_ADMIN_PASSWORD` 覆盖。**不要把真实生产密码写进本目录或提交。**
- 截图输出到 `gui-test-screenshots/e2e-py/`。
- 用例自清理：创建的文章/评论在用例内删除；连续多轮运行可能触发登录限流
  （10 次/5min/IP），可清 Redis `blog:ratelimit:*` 或等 5 分钟。
- 退出码：存在「意外失败」时为 1；仅「已知缺陷复现」（known_issue 标注）不算失败。
