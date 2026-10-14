ALTER TABLE `articles`
  DROP KEY `article_featured_published_at`,
  DROP COLUMN `featured`,
  DROP COLUMN `cover_url`;
