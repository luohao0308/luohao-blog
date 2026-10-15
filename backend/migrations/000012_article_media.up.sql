ALTER TABLE `articles`
  ADD COLUMN `cover_url` varchar(255) COLLATE utf8mb4_bin NOT NULL DEFAULT '' AFTER `like_count`,
  ADD COLUMN `featured` boolean NOT NULL DEFAULT FALSE AFTER `cover_url`,
  ADD KEY `article_featured_published_at` (`featured`, `published_at`);
