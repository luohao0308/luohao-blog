-- Categories: admin-curated single-level taxonomy; an article belongs to at
-- most one category (articles.category_id, NULL = uncategorized). The table
-- is additive, so this migration is safe to run while the old code serves.

CREATE TABLE IF NOT EXISTS `categories` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `created_at` timestamp NOT NULL,
  `updated_at` timestamp NOT NULL,
  `slug` varchar(255) COLLATE utf8mb4_bin NOT NULL,
  `name` varchar(255) COLLATE utf8mb4_bin NOT NULL,
  `sort` int NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `slug` (`slug`),
  KEY `category_updated_at` (`updated_at`),
  KEY `category_sort` (`sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

ALTER TABLE `articles`
  ADD COLUMN `category_id` char(36) COLLATE utf8mb4_bin NULL,
  ADD KEY `article_category_id` (`category_id`),
  ADD CONSTRAINT `articles_category_id` FOREIGN KEY (`category_id`) REFERENCES `categories` (`id`) ON DELETE SET NULL;
