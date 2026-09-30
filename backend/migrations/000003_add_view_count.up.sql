-- Public view counter for articles. Incremented by the view-report endpoint
-- after the Redis 24h per-client dedup window admits the request; the article
-- write path never touches it.

ALTER TABLE `articles`
  ADD COLUMN `view_count` bigint unsigned NOT NULL DEFAULT 0;
