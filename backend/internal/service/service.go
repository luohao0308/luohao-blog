package service

import "github.com/google/wire"

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(NewOfficialAccountService, NewArticleService, NewCategoryService, NewProjectService, NewAboutService, NewCollectionService, NewCommentService, NewSubscriberService, NewAuthServiceWithEmail, NewUserService, NewMediaService, NewArticleSearchService, NewChatService)
