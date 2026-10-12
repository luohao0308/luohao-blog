package biz

import "github.com/google/wire"

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewWechatSyncUsecase, NewArticleUsecaseWithWechat, NewCategoryUsecase, NewCommentUsecase, NewSubscriberUsecase, NewUserUsecase, NewAvatarUsecase, NewAuthUsecase, NewWechatUsecase, NewChatUsecase)
