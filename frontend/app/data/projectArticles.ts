// Editorial relationships, resolved against the public API before rendering.
// A slug here never makes a draft public or creates a placeholder article.
export const projectArticleSlugs: Record<string, string[]> = {
  'dev-workflow': ['dev-workflow-project', 'ai-delivery-guard'],
  'forge-harness': ['forge-harness-project'],
  'aigc': ['aigc-backend-delivery'],
  'luohao-blog': ['blog-article-lifecycle', 'article-cas-revisions-case-study', 'article-search-sync-case-study', 'fullstack-smoke-pitfalls', 'ent-m2m-edge-case-study', 'mysql-varchar-191-case-study'],
  'job-application-workbench': ['opportunity-workbench-architecture'],
}
