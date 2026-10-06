// rich-text 只认白名单标签与内联 style，不解析 class。后端 content_html
// 面向 Web 端输出，这里给高频块级元素补内联样式；文字排版颜色等可继承
// 属性由页面容器兜底。
export function decorateArticleHtml(html: string): string {
  return html
    .replace(/<img\b/gi, '<img style="max-width:100%;border-radius:8px;vertical-align:middle;" ')
    .replace(/<pre\b/gi, '<pre style="background:#f6f8fa;padding:12px;border-radius:8px;overflow-x:auto;white-space:pre;font-size:13px;line-height:1.6;" ')
    .replace(/<code\b/gi, '<code style="font-family:Menlo,Consolas,monospace;" ')
    .replace(/<blockquote\b/gi, '<blockquote style="border-left:4px solid #d1d5db;margin:8px 0;padding:2px 12px;color:#6b7280;" ')
    .replace(/<table\b/gi, '<table style="border-collapse:collapse;width:100%;" ')
    .replace(/<(td|th)\b/gi, '<$1 style="border:1px solid #e5e7eb;padding:6px 8px;" ')
    .replace(/<a\b/gi, '<a style="color:#2563eb;" ')
}
