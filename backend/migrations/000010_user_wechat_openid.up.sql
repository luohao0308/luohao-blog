-- WeChat mini-program identity per account (T-011/S2). NULL until an account
-- is bound through the wechat login flow; the unique index lets one openid
-- attach to exactly one account while MySQL happily stores many NULLs. The
-- mirror column lives in the ent User schema (wechat_openid). Both sides
-- must change together (schema drift debt).

ALTER TABLE `users`
  ADD COLUMN `wechat_openid` varchar(64) COLLATE utf8mb4_bin NULL,
  ADD UNIQUE INDEX `users_wechat_openid` (`wechat_openid`);
