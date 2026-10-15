ALTER TABLE `users`
  ADD COLUMN `email_verified` boolean NOT NULL DEFAULT FALSE AFTER `avatar_url`,
  ADD COLUMN `credential_version` bigint unsigned NOT NULL DEFAULT 0 AFTER `email_verified`;

CREATE TABLE `account_email_tokens` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `created_at` timestamp NOT NULL,
  `updated_at` timestamp NOT NULL,
  `user_id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `purpose` varchar(16) COLLATE utf8mb4_bin NOT NULL,
  `token_hash` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `credential_version` bigint unsigned NOT NULL DEFAULT 0,
  `expires_at` timestamp NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `account_email_token_hash` (`token_hash`),
  UNIQUE KEY `account_email_token_user_purpose` (`user_id`,`purpose`),
  KEY `account_email_token_expires_at` (`expires_at`),
  CONSTRAINT `account_email_token_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
