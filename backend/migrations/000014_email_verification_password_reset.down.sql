DROP TABLE IF EXISTS `account_email_tokens`;

ALTER TABLE `users`
  DROP COLUMN `credential_version`,
  DROP COLUMN `email_verified`;
