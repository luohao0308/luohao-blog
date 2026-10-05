-- Site-relative avatar path per account, served by the GetAvatar route.
-- Empty until the first upload; the mirror column lives in the ent User
-- schema (avatar_url). Both sides must change together (schema drift debt).

ALTER TABLE `users`
  ADD COLUMN `avatar_url` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '';
