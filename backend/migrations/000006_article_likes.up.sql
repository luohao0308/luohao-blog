-- Public like counter for articles. Incremented by the like endpoint after
-- the Redis 24h per-client dedup window admits the request; the article
-- write path never touches it. Same shape as view_count (000003).

ALTER TABLE `articles`
  ADD COLUMN `like_count` bigint unsigned NOT NULL DEFAULT 0;
