CREATE TABLE `article_revisions` (
 `id` char(36) NOT NULL,
 `article_id` char(36) NOT NULL,
 `version` bigint unsigned NOT NULL,
 `action` varchar(32) NOT NULL,
 `actor_id` char(36) NULL,
 `created_at` timestamp NOT NULL,
 `snapshot` json NOT NULL,
 PRIMARY KEY (`id`), UNIQUE KEY `articlerevision_article_id_version` (`article_id`,`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
