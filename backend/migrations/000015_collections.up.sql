CREATE TABLE `collections` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `saved_at` timestamp NOT NULL,
  `user_id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `article_id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `article_slug` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `collection_user_article` (`user_id`,`article_id`),
  KEY `collection_user_saved_slug_id` (`user_id`,`saved_at`,`article_slug`,`id`),
  KEY `collection_article_id` (`article_id`),
  CONSTRAINT `collection_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `collection_article_id` FOREIGN KEY (`article_id`) REFERENCES `articles` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
