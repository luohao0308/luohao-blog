-- Author account for comments: the CreateComment endpoint now requires a
-- signed-in account and stores its id. NULL keeps legacy anonymous rows
-- (closed-registration era) readable with their stored display name. Mirror
-- column lives in the ent Comment schema (user_id); both sides change
-- together (schema drift debt).

ALTER TABLE `comments`
  ADD COLUMN `user_id` char(36) COLLATE utf8mb4_bin NULL DEFAULT NULL;
