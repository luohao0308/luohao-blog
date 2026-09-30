-- Author accounts. Registration is closed: accounts are created by the seed
-- command only. role follows the biz.UserRole values (1 = admin, 2 = reader).

CREATE TABLE IF NOT EXISTS `users` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `created_at` timestamp NOT NULL,
  `updated_at` timestamp NOT NULL,
  `email` varchar(255) COLLATE utf8mb4_bin NOT NULL,
  `password_hash` varchar(255) COLLATE utf8mb4_bin NOT NULL,
  `display_name` varchar(255) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `role` int NOT NULL DEFAULT '1',
  PRIMARY KEY (`id`),
  UNIQUE KEY `email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
