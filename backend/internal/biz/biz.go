package biz

import "github.com/google/wire"

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewWechatSyncUsecase, NewArticleUsecaseWithWechat, NewCategoryUsecase, NewProjectUsecase, NewAboutUsecase, NewCollectionUsecase, NewCommentUsecase, NewSubscriberUsecase, NewUserUsecase, NewAvatarUsecase, NewMediaUsecase, NewAuthUsecaseWithEmail, NewAccountEmailUsecase, NewWechatUsecase, NewChatUsecase)
