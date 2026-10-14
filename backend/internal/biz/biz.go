package biz

import "github.com/google/wire"

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewWechatSyncUsecase, NewArticleUsecaseWithWechat, NewCategoryUsecase, NewProjectUsecase, NewAboutUsecase, NewCommentUsecase, NewSubscriberUsecase, NewUserUsecase, NewAvatarUsecase, NewMediaUsecase, NewAuthUsecase, NewWechatUsecase, NewChatUsecase)
