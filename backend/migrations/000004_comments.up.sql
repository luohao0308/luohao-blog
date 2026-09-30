-- Visitor comments, pre-moderated: new rows start pending and only approved
-- rows appear in public reads. Commenters are anonymous (registration is
-- closed); moderation happens in the admin panel.

CREATE TABLE IF NOT EXISTS `comments` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `created_at` timestamp NOT NULL,
  `updated_at` timestamp NOT NULL,
  `article_slug` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `display_name` varchar(32) COLLATE utf8mb4_bin NOT NULL,
  `content` text COLLATE utf8mb4_bin NOT NULL,
  `status` int NOT NULL DEFAULT '1',
  PRIMARY KEY (`id`),
  KEY `comments_article_slug` (`article_slug`),
  KEY `comments_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
