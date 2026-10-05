package service

import "github.com/google/wire"

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(NewArticleService, NewCategoryService, NewCommentService, NewSubscriberService, NewAuthService, NewUserService, NewArticleSearchService, NewChatService)
