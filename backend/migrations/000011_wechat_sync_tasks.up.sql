-- Durable WeChat Official Account publication outbox. Each row owns an
-- immutable article snapshot and workflow state; credentials remain in
-- runtime configuration and are never stored here. The application-generated
-- UUID is the sole idempotency key because it incorporates account/config
-- inputs in addition to the article version.

CREATE TABLE `wechat_sync_tasks` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `created_at` timestamp NOT NULL,
  `updated_at` timestamp NOT NULL,
  `slug` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `content_hash` varchar(128) COLLATE utf8mb4_bin NOT NULL,
  `mode` varchar(32) COLLATE utf8mb4_bin NOT NULL,
  `account_fingerprint` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `state` varchar(32) COLLATE utf8mb4_bin NOT NULL,
  `article_title` longtext COLLATE utf8mb4_bin NOT NULL,
  `article_summary` longtext COLLATE utf8mb4_bin NOT NULL,
  `article_content_html` longtext COLLATE utf8mb4_bin NOT NULL,
  `article_source_url` longtext COLLATE utf8mb4_bin NOT NULL,
  `article_author` varchar(255) COLLATE utf8mb4_bin NOT NULL,
  `article_thumb_media_id` varchar(255) COLLATE utf8mb4_bin NOT NULL,
  `draft_id` varchar(255) COLLATE utf8mb4_bin NULL,
  `remote_id` varchar(255) COLLATE utf8mb4_bin NULL,
  `article_url` longtext COLLATE utf8mb4_bin NULL,
  `last_error` varchar(255) COLLATE utf8mb4_bin NULL,
  `attempts` bigint NOT NULL DEFAULT '0',
  `next_attempt_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `wechatsynctask_updated_at` (`updated_at`),
  KEY `wechatsynctask_slug_content_hash_mode` (`slug`,`content_hash`,`mode`),
  KEY `wechatsynctask_state_updated_at` (`state`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
