ALTER TABLE `articles`
  DROP FOREIGN KEY `articles_category_id`,
  DROP COLUMN `category_id`;

DROP TABLE IF EXISTS `categories`;
