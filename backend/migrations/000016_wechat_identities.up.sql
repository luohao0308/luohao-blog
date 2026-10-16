CREATE TABLE `wechat_identities` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `user_id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `app_id` varchar(32) COLLATE utf8mb4_bin NOT NULL,
  `openid` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `created_at` timestamp NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `wechat_identity_app_openid` (`app_id`,`openid`),
  UNIQUE KEY `wechat_identity_user_app` (`user_id`,`app_id`),
  CONSTRAINT `wechat_identity_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
