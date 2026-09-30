-- Baseline: schema as shipped with M1 (article/tag model). Fresh databases
-- start here; existing M1 databases created by ent auto_migrate are already
-- in this state, so the migration is written to be re-runnable (IF NOT EXISTS).
-- NOTE: golang-migrate tracks applied versions in schema_migrations; the
-- IF NOT EXISTS guards only protect against dirty local re-runs.

CREATE TABLE IF NOT EXISTS `articles` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `created_at` timestamp NOT NULL,
  `updated_at` timestamp NOT NULL,
  `slug` varchar(255) COLLATE utf8mb4_bin NOT NULL,
  `title` longtext COLLATE utf8mb4_bin,
  `summary` longtext COLLATE utf8mb4_bin,
  `content_md` longtext COLLATE utf8mb4_bin,
  `content_html` longtext COLLATE utf8mb4_bin,
  `status` int NOT NULL DEFAULT '1',
  `published_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `slug` (`slug`),
  KEY `article_updated_at` (`updated_at`),
  KEY `article_status` (`status`),
  KEY `article_published_at` (`published_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE IF NOT EXISTS `tags` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `created_at` timestamp NOT NULL,
  `updated_at` timestamp NOT NULL,
  `name` varchar(255) COLLATE utf8mb4_bin NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `name` (`name`),
  KEY `tag_updated_at` (`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE IF NOT EXISTS `article_tags` (
  `article_id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `tag_id` char(36) COLLATE utf8mb4_bin NOT NULL,
  PRIMARY KEY (`article_id`,`tag_id`),
  KEY `article_tags_tag_id` (`tag_id`),
  CONSTRAINT `article_tags_article_id` FOREIGN KEY (`article_id`) REFERENCES `articles` (`id`) ON DELETE CASCADE,
  CONSTRAINT `article_tags_tag_id` FOREIGN KEY (`tag_id`) REFERENCES `tags` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
