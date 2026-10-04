-- Subscribers: email registration for future article notifications. Actual
-- sending is not wired yet; this table is the ledger the future sender reads.
-- Emails are stored lowercased and unique; re-subscribing is idempotent.

CREATE TABLE IF NOT EXISTS `subscribers` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `created_at` timestamp NOT NULL,
  `updated_at` timestamp NOT NULL,
  `email` varchar(191) COLLATE utf8mb4_bin NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `email` (`email`),
  KEY `subscriber_updated_at` (`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
